package main

// @controllers.go contains all core features to pick up a new job and schedule and start its processing.

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
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
					// so, if it is a short job dump its output. Then delete it.
					if !job.islong {
						filenameSuffix := fmt.Sprintf("%02d%02d%02d.%s.txt", job.submittime.Year(), job.submittime.Month(), job.submittime.Day(), job.id)
						filename := filepath.Join(Config.JobsOutputsFolder, filenameSuffix)
						if err := os.WriteFile(filename, job.result.Bytes(), 0644); err != nil {
							deletedjobslog.Printf("[%s] [%05d] cleanup routine failed to dump job output before deletion.\n", id, job.id)
						}
					}

					delete(globalJobsResults, id)
					deletedjobslog.Printf("[%s] [%05d] cleanup routine removed job from the results queue.\n", id, job.id)
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
	job.pid = cmd.Process.Pid
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

	job.iscompleted = true
	job.endtime = time.Now().UTC()

	if err != nil {
		// timeout not reached - but an error occured during execution
		jobslog.Printf("[%s] [%05d] error occured during the processing of the job - errmsg: %v\n", job.id, job.pid, err)
		job.issuccess = false
		job.errormsg = err.Error()
		// lets get the exit code.
		if exitError, ok := err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			job.exitcode = ws.ExitStatus()
		}
	}

	if err == nil && !stopped {
		// exited from select loop due to other reason than job stop request.
		jobslog.Printf("[%s] [%05d] completed the processing of the job\n", job.id, job.pid)
		job.issuccess = true
		// success, exitCode should be 0
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		job.exitcode = ws.ExitStatus()
	}

	if err == nil && stopped {
		// exited from select loop due to job stop request.
		jobslog.Printf("[%s] [%05d] stopped the processing of the job\n", job.id, job.pid)
		job.issuccess = true
		// no exit code for killed process - lets leave it to -1 for reference.
	}
}
