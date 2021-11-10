package main

// @controllers.go contains all core features to pick up a new job and schedule and start its processing.

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// cleanupMapResults runs every <interval> hours and delete completed job which
//  reached <maxcount> times fetching or completed for more than <deadtime> hours.
func cleanupMapResults(interval int, maxcount int, deadtime int, exit <-chan struct{}, wg *sync.WaitGroup) {
	log.Println("started goroutine to cleanup stalling jobs ...")
	defer wg.Done()
	for {
		select {
		case <-exit:
			// worker to stop - so leave this goroutine.
			return
		case <-time.After(time.Duration(interval) * time.Hour):
			// waiting time passed so lock the map and loop over each (*job).
			mapLock.Lock()
			for id, job := range globalJobsResults {
				if job.iscompleted && (job.fetchcount > maxcount || (time.Since(job.endtime) > (time.Duration(deadtime) * time.Minute))) {
					// job terminated and reached deadtime or max fetch count.
					// so, if it is a short job backup its output. Then delete it.
					if !job.islong {
						filenameSuffix := fmt.Sprintf("%02d%02d%02d.%s.txt", job.submittime.Year(), job.submittime.Month(), job.submittime.Day(), job.id)
						filename := filepath.Join(Config.JobsOutputsBackupsFolder, filenameSuffix)
						if err := os.WriteFile(filename, job.result.Bytes(), 0644); err != nil {
							deletedjobslog.Printf("[%s] [%05d] cleanup routine failed to backup job output before deletion.\n", id, job.pid)
						}
					}

					delete(globalJobsResults, id)
					deletedjobslog.Printf("[%s] [%05d] cleanup routine removed job from the results queue.\n", id, job.pid)
				}
			}
			mapLock.Unlock()
		}
		// relax to avoid cpu spike into this indefinite loop.
		time.Sleep(100 * time.Millisecond)
	}
}

// jobsMonitor watches the global jobs queue and spin up a separate executor to handle the task.
func jobsMonitor(exit <-chan struct{}, wg *sync.WaitGroup) {
	log.Println("started goroutine to monitor submitted jobs queue ...")
	defer wg.Done()
	ctx, cancel := context.WithCancel(context.Background())
	// indefinite loop on the jobs channel.
	for {
		select {
		case job := <-globalJobsQueue:
			go executeJob(job, ctx)
		case <-exit:
			// signal to stop monitoring - notify all executors to stop as well.
			cancel()
			return
		}
		// pause the infinite loop to avoid cpu spike.
		time.Sleep(time.Duration(100) * time.Millisecond)
	}
}

// executeJob takes a job structure and will execute the task and add the result to the global results bus.
func executeJob(job *Job, ctx context.Context) {
	job.starttime = time.Now().UTC()
	var err error
	jobctx := ctx
	if job.islong {
		// long running job so timeout option provided into minutes.
		jobctx, _ = context.WithTimeout(ctx, time.Duration(job.timeout)*time.Minute)
	} else {
		// short running job so timeout option provided into seconds.
		jobctx, _ = context.WithTimeout(ctx, time.Duration(job.timeout)*time.Second)
	}

	jobslog.Printf("[%s] [%05d] starting the processing of the job\n", job.id, job.pid)
	var cmd *exec.Cmd

	// command syntax for windows platform.
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(jobctx, "cmd", "/C", job.task)
	} else {
		// syntax for linux-based platforms.
		cmd = exec.CommandContext(jobctx, Config.DefaultLinuxShell, "-c", job.task)
	}

	// combine standard process pipes, stdout & stderr.
	cmd.Stderr = cmd.Stdout

	if job.islong {
		// long running job.
		if job.stream && !job.dump {
			// with stream over websocket only. store the standard pipe for streaming.
			job.outpipe, err = cmd.StdoutPipe()
			if err != nil {
				// job should be streaming so abort this job scheduling.
				jobslog.Printf("[%s] [%05d] error occured during the scheduling of the job - errmsg: %v\n", job.id, job.pid, err)
				job.setFailedInfos(time.Now().UTC(), err.Error())
				return
			}

		} else if job.stream && job.dump {
			// with stream over websocket and to disk file.
			// construct the filename based on job submitted time and its id.
			filenameSuffix := fmt.Sprintf("%02d%02d%02d.%s.txt", job.submittime.Year(), job.submittime.Month(), job.submittime.Day(), job.id)
			job.filename = filepath.Join(Config.JobsOutputsFolder, filenameSuffix)
			file, err := os.OpenFile(job.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
			if err != nil {
				// cannot satisfy the output dumping so abort the process.
				jobslog.Printf("[%s] [%05d] failed to create or open saving file for the job - errmsg: %v\n", job.id, job.pid, err)
				jobslog.Printf("[%s] [%05d] error occured during the scheduling of the job - errmsg: %v\n", job.id, job.pid, err)
				job.setFailedInfos(time.Now().UTC(), err.Error())
				return
			}
			defer file.Close()
			// duplicate the output stream for streaming to the disk file
			// and keep the second pipe for user streaming over websocket.
			outpipe, err := cmd.StdoutPipe()
			if err != nil {
				// job should be streaming so abort this job scheduling.
				jobslog.Printf("[%s] [%05d] failed to get standard output pipe of the job - errmsg: %v\n", job.id, job.pid, err)
				jobslog.Printf("[%s] [%05d] error occured during the scheduling of the job - errmsg: %v\n", job.id, job.pid, err)
				job.setFailedInfos(time.Now().UTC(), err.Error())
				return
			}

			job.outstream = io.TeeReader(outpipe, file)

		} else if !job.stream && job.dump {
			// only stream output to disk file. create/open file and use it as process pipe.
			filenameSuffix := fmt.Sprintf("%02d%02d%02d.%s.txt", job.submittime.Year(), job.submittime.Month(), job.submittime.Day(), job.id)
			job.filename = filepath.Join(Config.JobsOutputsFolder, filenameSuffix)
			file, err := os.OpenFile(job.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
			if err != nil {
				// cannot satisfy the output dumping so abort the process.
				jobslog.Printf("[%s] [%05d] failed to create or open saving file for the job - errmsg: %v\n", job.id, job.pid, err)
				jobslog.Printf("[%s] [%05d] error occured during the scheduling of the job - errmsg: %v\n", job.id, job.pid, err)
				job.setFailedInfos(time.Now().UTC(), err.Error())
				return
			}
			defer file.Close()
			cmd.Stdout = file
		} else {
			// should not wanted but if happened, then set outputs to dev/null.
			cmd.Stdout = nil
		}

	} else {
		// short running job so redirect the output to <result memory buffer>.
		cmd.Stdout = job.result
	}

	// asynchronously starting the job.
	if err := cmd.Start(); err != nil {
		// no need to continue - add job stats to map.
		jobslog.Printf("[%s] [%05d] failed to start the job - errmsg: %v\n", job.id, job.pid, err)
		job.setFailedInfos(time.Now().UTC(), err.Error())
		return
	}
	// job process started.
	job.lock.Lock()
	job.pid = cmd.Process.Pid
	job.lock.Unlock()
	jobslog.Printf("[%s] [%05d] started the processing of the job\n", job.id, job.pid)
	// var err error
	done := make(chan error)

	go func() {
		done <- cmd.Wait()
	}()

	err = nil
	// track if job stop was requested or not.
	stopped := false
	// block on select until one case hits.
	select {

	case <-jobctx.Done():

		switch jobctx.Err() {

		case context.DeadlineExceeded:
			// timeout reached - so try to kill the job process.
			jobslog.Printf("[%s] [%05d] timeout reached. failed to complete the processing of the job\n", job.id, job.pid)
		case context.Canceled:
			// context cancellation triggered.
			jobslog.Printf("[%s] [%05d] cancellation triggered. aborted the processing of the job\n", job.id, job.pid)
		}

		// kill the process and exit from this function.
		if perr := cmd.Process.Kill(); perr != nil {
			jobslog.Printf("[%s] [%05d] failed to kill the associated process of the job - errmsg: %v\n", job.id, job.pid, perr)
		} else {
			jobslog.Printf("[%s] [%05d] succeeded to kill the associated process of the job\n", job.id, job.pid)
		}
		// leave the select loop.
		break

	case <-job.stop:
		stopped = true
		// stop requested by user. kill the process and exit from this function.
		jobslog.Printf("[%s] [%05d] stopping the processing of the job\n", job.id, job.pid)
		if perr := cmd.Process.Kill(); perr != nil {
			jobslog.Printf("[%s] [%05d] failed to kill the associated process of the job - errmsg: %v\n", job.id, job.pid, perr)
		} else {
			jobslog.Printf("[%s] [%05d] succeeded to kill the associated process of the job\n", job.id, job.pid)
		}
		// leave the select loop.
		break

	case err = <-done:
		// task completed before timeout. exit from select loop.
		break
	}
	job.lock.Lock()
	job.iscompleted = true
	job.endtime = time.Now().UTC()
	job.lock.Unlock()

	if err != nil {
		// timeout not reached - but an error occured during execution
		jobslog.Printf("[%s] [%05d] error occured during the processing of the job - errmsg: %v\n", job.id, job.pid, err)
		job.lock.Lock()
		job.issuccess = false
		job.errormsg = err.Error()
		job.lock.Unlock()
		// lets get the exit code.
		if exitError, ok := err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			job.exitcode = ws.ExitStatus()
		}
	}

	if err == nil {
		// job completed without error.
		job.lock.Lock()
		job.issuccess = true
		job.lock.Unlock()
		if !stopped {
			// exited from select loop due to other reason than job stop request.
			jobslog.Printf("[%s] [%05d] completed the processing of the job\n", job.id, job.pid)
			// success, exitCode should be 0
			ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
			job.lock.Lock()
			job.exitcode = ws.ExitStatus()
			job.lock.Unlock()
		} else {
			// job completed with stop request.
			jobslog.Printf("[%s] [%05d] stopped the processing of the job\n", job.id, job.pid)
			// no exit code for killed process - we leave it to default (-1) for reference.
		}
	}
}

// global channel to receive os signals.
var signalsChan = make(chan os.Signal, 2)

// handleSignals is a function that processes common and custom signals types.
func handleSignals(exit chan struct{}, wg *sync.WaitGroup) {
	log.Println("started goroutine for exit signal handling ...")
	defer wg.Done()
	// if 1 means ongoing stack trace dumping.
	var dump uint32
	// signalsChan := make(chan os.Signal, 1)
	// setup the list of supported signals types.
	signal.Notify(signalsChan, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL,
		syscall.SIGTERM, syscall.SIGHUP, os.Interrupt, os.Kill, syscall.SIGABRT)

	for sigType := range signalsChan {

		switch sigType {

		case syscall.SIGQUIT:
			// SIGQUIT 	3 - QUIT character (Ctrl+/) to trigger.
			// received stack trace dump signal.

			if atomic.LoadUint32(&dump) == 1 {
				// there is ongoing trace dumping.
				log.Printf("cannot execute trace dump command [signal: %v] because there is ongoing diagnostics", sigType)
				continue
			}

			atomic.StoreUint32(&dump, 1)
			go func() {
				diagnosticsId := generateDiagnosticsRequestID(time.Now())
				log.Printf("received trace dump command [signal: %v]. collecting diagnostics <%s>", sigType, diagnosticsId)
				if ok := collectStackTrace(Config.DiagnosticsFoldersLocation, diagnosticsId, Config.CpuProfilingDuration); !ok {
					log.Printf("error occured during stack traces dump for diagnostics <%s>", diagnosticsId)
				}
				atomic.StoreUint32(&dump, 0)
			}()

		default:
			// must be an exit signal.
			log.Printf("received exit command [signal: %v]. stopping worker service.", sigType)
			signal.Stop(signalsChan)
			// perform cleanup action : remove pid file.
			os.Remove(Config.WorkerPidFilePath)
			// close quit channel, so jobs monitor goroutine will be notified.
			close(exit)
			return
		}
	}
}

// collectStackTrace executes each on-demand runtime diagnostics data collection.
// It creates a unique folder before and store the profiles output into.
func collectStackTrace(diagsFolderLocation, diagid string, duration int) bool {

	folderPath := filepath.Join(diagsFolderLocation, diagid)
	if err := createFolder(folderPath); err != nil {
		log.Printf("failed to create folder for diagnostics reporting [%s] - errmsg: %v\n", err, diagid)
		return false
	}

	allgood := true

	// perform CPU profiling for configured seconds.
	fc, err := os.Create(filepath.Join(folderPath, "cpu.out"))
	if err != nil {
		allgood = false
		log.Printf("failed to create <cpu.out> for diagnostics reporting [%s] - errmsg: %v", diagid, err)
	} else {
		pprof.StartCPUProfile(fc)
		time.Sleep(time.Duration(duration) * time.Second)
		pprof.StopCPUProfile()
		fc.Close()
	}

	// goroutine - stack traces of all current goroutines.
	fg, err := os.Create(filepath.Join(folderPath, "goroutine.out"))
	if err != nil {
		allgood = false
		log.Printf("failed to create <goroutine.out> for diagnostics reporting [%s] - errmsg: %v", diagid, err)
	} else {
		if err = pprof.Lookup("goroutine").WriteTo(fg, 2); err != nil {
			allgood = false
			log.Printf("failed to dump goroutine profile for diagnostics reporting [%s] - errmsg: %v", diagid, err)
		}
		fg.Close()
	}

	// threadcreate - stack traces that led to the creation of new OS threads.
	ft, err := os.Create(filepath.Join(folderPath, "threads.out"))
	if err != nil {
		allgood = false
		log.Printf("failed to create <threads.out> for diagnostics reporting [%s] - errmsg: %v", diagid, err)
	} else {
		if err = pprof.Lookup("threadcreate").WriteTo(ft, 1); err != nil {
			allgood = false
			log.Printf("failed to dump os threads profile for diagnostics reporting [%s] - errmsg: %v", diagid, err)
		}
		ft.Close()
	}

	// block - stack traces that led to blocking on synchronization primitives.
	fb, err := os.Create(filepath.Join(folderPath, "block.out"))
	if err != nil {
		allgood = false
		log.Printf("failed to create <block.out> for diagnostics reporting [%s] - errmsg: %v", diagid, err)
		return false
	} else {
		if err = pprof.Lookup("block").WriteTo(fb, 1); err != nil {
			allgood = false
			log.Printf("failed to dump block profile for diagnostics reporting [%s] - errmsg: %v", diagid, err)
		}
		fb.Close()
	}

	// mutex - stack traces of holders of contended mutexes.
	fm, err := os.Create(filepath.Join(folderPath, "mutex.out"))
	if err != nil {
		allgood = false
		log.Printf("failed to create <mutex.out> for diagnostics reporting [%s] - errmsg: %v", diagid, err)
	} else {
		if err = pprof.Lookup("mutex").WriteTo(fm, 1); err != nil {
			allgood = false
			log.Printf("failed to dump mutex profile for diagnostics reporting [%s] - errmsg: %v", diagid, err)
		}
		fm.Close()
	}

	// trigger garbage collection to materialize all statistics.
	runtime.GC()
	// heap - a sampling of memory allocations of live objects.
	// debug code is 0 bcz we want to use <go tool pprof> on it.
	fh, err := os.Create(filepath.Join(folderPath, "memory.out"))
	if err != nil {
		allgood = false
		log.Printf("failed to create <memory.out> for diagnostics reporting [%s] - errmsg: %v", diagid, err)
	} else {
		if err = pprof.Lookup("heap").WriteTo(fh, 0); err != nil {
			allgood = false
			log.Printf("failed to dump heap profile for diagnostics reporting [%s] - errmsg: %v", diagid, err)
		}
		fh.Close()
	}

	// allocs - a sampling of all past memory allocations.
	fa, err := os.Create(filepath.Join(folderPath, "allocs.out"))
	if err != nil {
		allgood = false
		log.Printf("failed to create <allocs.out> for diagnostics reporting [%s] - errmsg: %v", diagid, err)
	} else {
		if err = pprof.Lookup("allocs").WriteTo(fa, 1); err != nil {
			allgood = false
			log.Printf("failed to dump allocations profile for diagnostics reporting [%s] - errmsg: %v", diagid, err)
		}
	}
	fa.Close()

	return allgood
}
