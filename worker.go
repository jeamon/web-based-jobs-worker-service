package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

// this cross-platform web & systems backend allows to execute multiple commands from shell with possibility
// to specify single execution timeout for all of the commands submitted. The commands should be submmitted
// from web requests. Each command output will be made available into the system memory for further retrieval.
// Each command submitted is considered as a unique job with unique identifier. This backend service could be
// started - stopped and restarted almost like a unix deamon. Each submitted job could be stopped based on its
// unique id. Same approach to check the status. You can even view all submitted jobs status. Finally, you can
// submit a single command and wait until its execution ends in order to view its result output immediately.
// Each start of the worker creates a folder to host all three logs files (web requests - jobs - jobs deletion).

// Version  : 1.0
// Author   : Jerome AMON
// Created  : 20 August 2021

const address = "0.0.0.0:8080"

// fixed path of server certs & private key.
const serverCertsPath = "certs/server.crt"
const serverPrivKeyPath = "certs/server.key"

// waiting time before program exit at failure.
const waitingTime = 3

// cleanup routine default values.
const maxcount = 10
const interval = 12
const maxage = 24

// default short task (islong = false) timeout in secs.
const shortJobTimeout = 3600

// default long task (islong = true) timeout in mins.
const longJobTimeout = 1440

// store shell name for linux-based system.
var shell string

// custom logger for program details.
var weblog *log.Logger

// custom logger for tracing jobs deletion.
var deletedjobslog *log.Logger

// custom logger for tracing jobs processing.
var jobslog *log.Logger

// file name to save deamon PID.
var pidFile = "deamon.pid"

// Job is a structure for each submitted task. A long running job output could only be streamed
// and/or dumped to a file if specified. It can last for 24hrs (1440 mins). A short running task
// output will only be saved into its memory buffer and can last for 1h (3600s).
type Job struct {
	id          string        // auto-generated 16 hexa job id.
	pid         int           // related job system process id.
	task        string        // full command syntax to be executed.
	islong      bool          // false:(short task) and true:(long task).
	memlimit    int           // maximum allowed memory usage in MB.
	cpulimit    int           // maximum cpu percentage allowed for the job.
	timeout     int           // in secs for short task and mins for long task.
	iscompleted bool          // specifies if job is running or not.
	issuccess   bool          // job terminated without error.
	exitcode    int           // 0: success or -1: default & stopped.
	errormsg    string        // store any error during execution.
	fetchcount  int           // number of times result queried.
	stop        chan struct{} // helps notify to kill the job process.
	outpipe     io.ReadCloser // job process default std output/error pipe.
	outstream   io.Reader     // makes available output for streaming.
	isstreaming bool          // if stream already being consumed over a websocket.
	lock        *sync.RWMutex // job level mutex for access synchronization.
	result      *bytes.Buffer // in-memory buffer to save output - getJobsOutputById().
	filename    string        // filename where to dump long running job output.
	dump        bool          // if true then save output to disk file.
	submittime  time.Time     // datetime job was submitted.
	starttime   time.Time     // datetime job was started.
	endtime     time.Time     // datetime job was terminated.
}

// jobs allocator buffer (max jobs to into batch).
const maxJobs = 10

// buffered (maxJobs) channel to hold submitted (*job).
var globalJobsQueue chan *Job

// store all submitted jobs after processed with job id as key.
var globalJobsResults map[string]*Job
var mapLock *sync.RWMutex

func initializer() {
	mapLock = &sync.RWMutex{}
	globalJobsQueue = make(chan *Job, maxJobs)
	if globalJobsResults == nil {
		globalJobsResults = make(map[string]*Job)
	}

	// get curent working directory of the program and update the pidfile with its full / absolute path.
	if workDir, err := os.Getwd(); err == nil {
		pidFile = filepath.Join(workDir, pidFile)
	}

	// for linux-based platform lets find the current shell binary path
	// if not environnement not set or empty we use "/bin/sh" as default.
	if runtime.GOOS != "windows" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
	}
}

// generateID uses rand from crypto module to generate random
// value into hexadecimal mode this value will be used as job id
func generateID() string {

	// randomly fill the 8 capacity slice of bytes
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// use current number of nanoseconds since January 1, 1970 UTC
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

// savePID is a function to create new file and put inside the pid value passed.
func savePID(pid int) error {
	f, err := os.Create(pidFile)
	if err != nil {
		log.Printf("failed to create pid file - errmsg: %v\n", err)
		return err
	}

	defer f.Close()

	if _, err := f.WriteString(strconv.Itoa(pid)); err != nil {
		log.Printf("failed to create pid file - errmsg: %v\n", err)
		return err
	}

	f.Sync()
	return nil
}

// getPID read the stored PID value stored into the local file and return it.
func getPID() (pid int, err error) {

	if _, err = os.Stat(pidFile); err != nil {
		return 0, err
	}

	// read the file content.
	data, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}

	// convert string-based value into integer.
	pid, err = strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("invalid PID value: %s", string(data))
	}
	return pid, nil
}

// sendUsage returns docs about the APIs.
func webHelp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	fmt.Fprintf(w, help)

	return
}

// removeDuplicateJobIds rebuilds the slice of job ids (string type) by verifying the format and deleting
// duplicate elements. In case there is no remaining valid id it returns true to ignore the request.
func removeDuplicateJobIds(ids *[]string) bool {

	if len(*ids) == 1 {
		if match, _ := regexp.MatchString(`[a-z0-9]{16}`, (*ids)[0]); !match {
			return true
		}
	}

	temp := make(map[string]struct{})
	for _, id := range *ids {
		if match, _ := regexp.MatchString(`[a-z0-9]{16}`, id); !match {
			continue
		}
		temp[id] = struct{}{}
	}
	*ids = nil
	*ids = make([]string, 0)
	for id, _ := range temp {
		*ids = append(*ids, id)
	}

	if len(*ids) == 0 {
		return true
	}

	return false
}

// instantCommandExecutor processes a single passed command/task and start streaming the result output immediately.
// In case a timeout value is passed the command will be lasted into that interval /execute?cmd=dir+/B&timeout=1
func instantCommandExecutor(w http.ResponseWriter, r *http.Request) {
	// try to setup the response as not buffered data. if succeeds it will be used to flush.
	f, ok := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	// expect one value for cmd and timeout query strings.
	// if multiple passed only first will be retreived.
	task := r.URL.Query().Get("cmd")
	// nothing or empty value provided.
	if len(task) == 0 {
		w.WriteHeader(400)
		fmt.Fprintf(w, fmt.Sprintf("\n[+] The request sent is malformed • Go to https://%v/ to view detailed instructions.", r.Host))
		return
	}
	// get the request context and make it cancellable. update if task submitted with timeout option.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if timeout, err := strconv.Atoi(r.URL.Query().Get("timeout")); err == nil && timeout > 0 {
		ctx, _ = context.WithTimeout(ctx, time.Duration(timeout)*time.Minute)
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// command syntax for windows platform.
		cmd = exec.CommandContext(ctx, "cmd", "/C", task)
	} else {
		// syntax for linux-based platforms.
		cmd = exec.CommandContext(ctx, shell, "-c", task)
	}

	// combine standart output and error pipes.
	cmd.Stderr = cmd.Stdout
	stdout, _ := cmd.StdoutPipe()

	// channel to signal all output sent.
	done := make(chan struct{})
	// scans the output in a line-by-line fashion and stream the content manually.
	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			fmt.Fprintf(w, scanner.Text()+"\n")
			if ok {
				f.Flush()
			}
		}
		// all data streamed. unblock the channel.
		done <- struct{}{}
	}()

	// start asynchronously the task.
	if err := cmd.Start(); err != nil {
		// no need to continue.
		w.WriteHeader(400)
		fmt.Fprint(w, "\n[+] failed to execute the command submitted. make sure it is valid and has all rights.")
		return
	}
	w.WriteHeader(200)
	// build an id for this job and save its pid.
	id := generateID()
	pid := cmd.Process.Pid

	select {
	// block until there is a hit.
	case <-ctx.Done():

		switch ctx.Err() {

		case context.DeadlineExceeded:
			// timeout reached.
			jobslog.Printf("[%s] [%05d] timeout reached. stopped the processing of the job\n", id, pid)
			fmt.Fprintf(w, "\n\nFinish. Timeout reached. Stopping the job process.")
			if ok {
				f.Flush()
			}
		case context.Canceled:
			// request context cancelled - probably user closed the browser tab.
			jobslog.Printf("[%s] [%05d] request cancelled. stopped the processing of the job\n", id, pid)
		}

		// kill the process and exit from this select loop.
		if perr := cmd.Process.Kill(); perr != nil {
			jobslog.Printf("[%s] [%05d] failed to kill the associated process of the job - errmsg: %v\n", id, pid, perr)
		} else {
			jobslog.Printf("[%16s] [%05d] succeeded to kill the associated process of the job\n", id, pid)
		}

		break

	case <-done:
		// all data sent.
		break
	}
	// wait until task completes.
	cmd.Wait()
	jobslog.Printf("[%s] [%05d] completed the processing of the job\n", id, pid)
}

// build a string made of dash symbol - used to display table.
func Dashs(count int) string {
	return strings.Repeat("-", count)
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
		cmd = exec.CommandContext(jobctx, shell, "-c", job.task)
	}

	// combine standard process pipes, stdout & stderr.
	cmd.Stderr = cmd.Stdout

	if job.islong && !job.dump {
		// long running job without saving to disk file so we only store the standard pipe for further streaming.
		job.outpipe, err = cmd.StdoutPipe()
		if err != nil {
			// job should be streaming so abort this job scheduling.
			job.endtime = time.Now().UTC()
			job.iscompleted = true
			jobslog.Printf("[%s] [%05d] error occured during the scheduling of the job - errmsg: %v\n", job.id, job.pid, err)
			job.issuccess = false
			job.errormsg = err.Error()
			return
		}
	} else if job.islong && job.dump {
		// long running job and user requested to save output to disk file.
		// construct the filename based on job submitted time and its id.
		job.filename = fmt.Sprintf("outputs/%02d%02d%02d.%s.txt", job.submittime.Year(), job.submittime.Month(), job.submittime.Day(), job.id)
		file, err := os.OpenFile(job.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			// cannot satisfy the output dumping so abort the process.
			jobslog.Printf("[%s] [%05d] failed to create or open saving file for the job - errmsg: %v\n", job.id, job.pid, err)
			job.endtime = time.Now().UTC()
			job.iscompleted = true
			jobslog.Printf("[%s] [%05d] error occured during the scheduling of the job - errmsg: %v\n", job.id, job.pid, err)
			job.issuccess = false
			job.errormsg = err.Error()
			return
		}
		defer file.Close()
		// duplicate the output stream for streaming to the disk file and keep the second for user streaming.
		outpipe, err := cmd.StdoutPipe()
		if err != nil {
			// job should be streaming so abort this job scheduling.
			job.endtime = time.Now().UTC()
			job.iscompleted = true
			jobslog.Printf("[%s] [%05d] error occured during the scheduling of the job - errmsg: %v\n", job.id, job.pid, err)
			job.issuccess = false
			job.errormsg = err.Error()
			return
		}
		job.outstream = io.TeeReader(outpipe, file)
	} else {
		// short running job so set the output to result memory buffer.
		cmd.Stdout = job.result
	}

	// asynchronously starting the job.
	if err := cmd.Start(); err != nil {
		// no need to continue - add job stats to map.
		job.endtime = time.Now().UTC()
		jobslog.Printf("[%s] [%05d] failed to start the job - errmsg: %v\n", job.id, job.pid, err)
		job.issuccess = false
		job.errormsg = err.Error()
		job.iscompleted = true
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

// scheduleShortRunningJobs handles user submitted short running jobs (<islong> field will be set to false)
// requests and build the required jobs structure for immediate processing scheduling.
func scheduleShortRunningJobs(w http.ResponseWriter, r *http.Request) {

	// try to setup the response as not buffered data. if succeeds it will be used to flush.
	f, ok := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	// parse all query strings.
	query := r.URL.Query()
	cmds, exist := query["cmd"]
	if !exist || len(cmds) == 0 {
		// request does not contains query string cmd.
		w.WriteHeader(400)
		fmt.Fprintf(w, "\n[+] Sorry, the request submitted is malformed. The expected format is :\n http://server-ip:8080jobs/short?cmd=xxx&cmd=xxx&mem=<value>&cpu=<value>&timeout=<value>\n")
		return
	}
	// default memory (megabytes) and cpu (percentage) limit values.
	memlimit := 100
	cpulimit := 10
	timeout := shortJobTimeout
	// extract only first value of mem and cpu query string.
	if m, err := strconv.Atoi(query.Get("mem")); err == nil && m > 0 {
		memlimit = m
	}

	if c, err := strconv.Atoi(query.Get("cpu")); err == nil && c > 0 {
		cpulimit = c
	}
	// retreive timeout parameter value and consider it if higher than 0.
	if t, err := strconv.Atoi(query.Get("timeout")); err == nil && t > 0 && t <= shortJobTimeout {
		timeout = t
	}
	w.WriteHeader(200)
	w.Write([]byte("\n[+] find below some details of the jobs submitted\n\n"))

	// format the display table.
	title := fmt.Sprintf("|%-4s | %-18s | %-5s | %-5s | %-7s | %-20s | %-30s |", "Nb", "Job ID", "Mem", "CPU%", "Timeout", "Submitted At [UTC]", "Command Syntax")
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(5), Dashs(5), Dashs(7), Dashs(20), Dashs(30)))
	if ok {
		f.Flush()
	}
	// build each job per command with resources limit values.
	for i, cmd := range cmds {
		job := &Job{
			id:          generateID(),
			pid:         0,
			task:        cmd,
			islong:      false,
			iscompleted: false,
			issuccess:   false,
			exitcode:    -1,
			errormsg:    "",
			fetchcount:  0,
			stop:        make(chan struct{}, 1),
			lock:        &sync.RWMutex{},
			result:      new(bytes.Buffer),
			dump:        false,
			memlimit:    memlimit,
			cpulimit:    cpulimit,
			timeout:     timeout,
			submittime:  time.Now().UTC(),
			starttime:   time.Time{},
			endtime:     time.Time{},
		}
		// register job to global map results so user can check status while job being executed.
		mapLock.Lock()
		globalJobsResults[job.id] = job
		mapLock.Unlock()
		// add this job to the processing queue.
		globalJobsQueue <- job
		jobslog.Printf("[%s] [%05d] scheduled the processing of the job\n", job.id, job.pid)
		// stream the added job details to user/client.
		fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-5d | %-5d | %-7d | %-20v | %-30s |", i+1, job.id, job.memlimit, job.cpulimit, job.timeout, (job.submittime).Format("2006-01-02 15:04:05"), truncateSyntax(job.task, 30)))
		fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(5), Dashs(5), Dashs(7), Dashs(20), Dashs(30)))
		if ok {
			f.Flush()
		}
	}
}

// stopJobsById allows to abort execution of one or multiple submitted jobs which are in running state
// (so their iscompleted field is false) - triggered for following pattern : /jobs/stop?id=xxx&id=xxx
func stopJobsById(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf8")

	// parse all query strings.
	query := r.URL.Query()
	ids, exist := query["id"]
	if !exist || len(ids) == 0 {
		// request does not contains query string id.
		w.WriteHeader(400)
		w.Write([]byte("\n[+] Hello • The request sent is malformed • The expected format is :\n http://server-ip:8080/jobs/stop?id=xx&id=xx\n"))
		return
	}

	// cleanup user submitted list of jobs ids.
	if ignore := removeDuplicateJobIds(&ids); ignore {
		// no remaining good job id.
		w.WriteHeader(400)
		w.Write([]byte("\n[+] Hello • The request sent does not contain any valid job id remaining after verification.\n"))
		return
	}
	// try to setup the response as not buffered data. if succeeds it will be used to flush.
	f, ok := w.(http.Flusher)
	w.WriteHeader(200)
	w.Write([]byte("\n[+] stopped jobs [non-existent or invalid ids will be ignored] - zoom in to fit the screen\n\n"))

	// format the display table.
	title := fmt.Sprintf("|%-4s | %-18s | %-14s | %-16s |", "Nb", "Job ID", "Was Running", "Stop Triggered")
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(14), Dashs(16)))
	if ok {
		f.Flush()
	}
	// retreive each job status and send.
	i := 0
	var errorsMessages string
	mapLock.RLock()
	for _, id := range ids {

		job, exist := globalJobsResults[id]

		if !exist {
			// job id does not exist.
			errorsMessages = errorsMessages + fmt.Sprintf("\n[-] Job ID [%s] does not exist - maybe wrong id or removed from store\n", id)
			continue
		}
		i += 1
		if job.iscompleted == false {
			// stream the added job details to user/client.
			fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-14s | %-16s |", i, job.id, "true", "true"))
			// add value to stop channel to trigger job stop.
			job.stop <- struct{}{}

		} else {
			// job already completed (not running).
			fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-14s | %-16s |", i, job.id, "false", "false"))
		}

		fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(14), Dashs(16)))
		if ok {
			f.Flush()
		}
	}
	mapLock.RUnlock()
	// send all errors collected.
	fmt.Fprintln(w, errorsMessages)
	if ok {
		f.Flush()
	}
}

// stopAllJobs triggers termination of all submitted jobs which are in running state
// (so their iscompleted field is false) - triggered for following pattern : /jobs/stop/.
func stopAllJobs(w http.ResponseWriter, r *http.Request) {
	// try to setup the response as not buffered data. if succeeds it will be used to flush.
	f, ok := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")

	w.WriteHeader(200)
	w.Write([]byte("\n[+] result of stopping all running jobs - zoom in to fit the screen\n\n"))

	// format the display table.
	title := fmt.Sprintf("|%-4s | %-18s | %-14s | %-16s |", "Nb", "Job ID", "Was Running", "Stop Triggered")
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(14), Dashs(16)))
	if ok {
		f.Flush()
	}
	// lock the map and iterate over to check each job status
	// (iscompleted) and fill the stop channel accordingly.
	i := 0
	mapLock.RLock()
	for _, job := range globalJobsResults {
		i += 1
		if job.iscompleted == false {
			// job still running -must be stopped.
			fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-14s | %-16s |", i, job.id, "true", "true"))
			// add value to stop channel to trigger job stop.
			job.stop <- struct{}{}

		} else {
			// job already completed (not running).
			fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-14s | %-16s |", i, job.id, "false", "false"))
		}

		fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(14), Dashs(16)))
		if ok {
			f.Flush()
		}
	}
	mapLock.RUnlock()
}

// restartJobsById allows to restart execution of one or multiple submitted jobs. If the job is in completed state
// it get restarted immediately. if it is in running state, the stop will just be triggered so user can restart it
// later. This is the URI to trigger this handler function: /jobs/restart?id=xxx&id=xxx
func restartJobsById(w http.ResponseWriter, r *http.Request) {
	// try to setup the response as not buffered data. if succeeds it will be used to flush.
	f, ok := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	// parse all query strings.
	query := r.URL.Query()
	ids, exist := query["id"]
	if !exist || len(ids) == 0 {
		// request does not contains query string id.
		w.WriteHeader(400)
		w.Write([]byte("\n[+] Sorry, the request sent is malformed. Please check details at https://server-ip:8080/"))
		return
	}

	// cleanup user submitted list of jobs ids.
	if ignore := removeDuplicateJobIds(&ids); ignore {
		// no remaining good job id.
		w.WriteHeader(400)
		w.Write([]byte("\n[+] Hello • The request sent does not contain any valid job id remaining after verification.\n"))
		return
	}

	w.WriteHeader(200)
	w.Write([]byte("\n[+] restart summary - retry or check status for not restarted jobs. zoom in to fit the screen\n\n"))
	if ok {
		f.Flush()
	}
	// format the display table.
	title := fmt.Sprintf("|%-4s | %-18s | %-14s | %-16s | %-12s |", "Nb", "Job ID", "Was Running", "Stop Triggered", "Restarted")
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(14), Dashs(16), Dashs(12)))
	if ok {
		f.Flush()
	}
	i := 0
	var errorsMessages string
	for _, id := range ids {

		mapLock.RLock()
		job, exist := globalJobsResults[id]
		mapLock.RUnlock()

		if !exist {
			// job id does not exist.
			errorsMessages = errorsMessages + fmt.Sprintf("\n[-] Job ID [%s] does not exist - maybe wrong id or removed from store\n", id)
			continue
		}
		i += 1
		if job.iscompleted == false {
			// job is still running. add value to stop channel to trigger job stop.
			job.stop <- struct{}{}
			jobslog.Printf("[%s] [%05d] restart requested for the running job. stop triggered for the job\n", job.id, job.pid)
			fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-14s | %-16s | %-12s |", i, job.id, "yes", "yes", "n/a"))

		} else {
			// job already completed (not running). reset and start it by adding to jobs queue.
			mapLock.Lock()
			resetCompletedJobInfos(job)
			mapLock.Unlock()
			globalJobsQueue <- job
			jobslog.Printf("[%s] [%05d] restart requested for the completed job. reset the details and scheduled the job\n", job.id, job.pid)
			fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-14s | %-16s | %-12s |", i, job.id, "no", "n/a", "yes"))
		}

		fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(14), Dashs(16), Dashs(12)))
		if ok {
			f.Flush()
		}
	}
	// send all errors collected.
	fmt.Fprintln(w, errorsMessages)
	if ok {
		f.Flush()
	}
}

// restartAllJobs triggers termination of all short running jobs and immediate start of all completed short jobs.
func restartAllJobs(w http.ResponseWriter, r *http.Request) {
	// try to setup the response as not buffered data. if succeeds it will be used to flush.
	f, ok := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	w.WriteHeader(200)
	w.Write([]byte("\n[+] restart summary - retry or check status for not restarted jobs. zoom in to fit the screen\n\n"))
	if ok {
		f.Flush()
	}
	// format the display table.
	title := fmt.Sprintf("|%-4s | %-18s | %-14s | %-16s | %-12s |", "Nb", "Job ID", "Was Running", "Stop Triggered", "Restarted")
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(14), Dashs(16), Dashs(12)))
	if ok {
		f.Flush()
	}
	// lock the map and iterate over to check each job status (iscompleted) then fill
	// the stop channel accordingly or reset before adding the job to processing queue.
	i := 0
	//wg := &sync.WaitGroup{}
	mapLock.Lock()
	for _, job := range globalJobsResults {

		if job.islong {
			// long running job can be restarted by its id, so when restartJobsById() invoked.
			continue
		}

		i += 1
		if job.iscompleted == false {
			// job is still running. add value to stop channel to trigger job stop.
			job.stop <- struct{}{}
			jobslog.Printf("[%s] [%05d] restart requested for the running job. stop triggered for the job\n", job.id, job.pid)
			fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-14s | %-16s | %-12s |", i, job.id, "yes", "yes", "n/a"))

		} else {
			// job already completed (not running). so reset and start it by adding to jobs queue in parallel.
			resetCompletedJobInfos(job)
			globalJobsQueue <- job
			jobslog.Printf("[%s] [%05d] restart requested for the completed job. reset the details and scheduled job onto processing queue\n", job.id, job.pid)
			fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-14s | %-16s | %-12s |", i, job.id, "no", "n/a", "yes"))
		}
		fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(14), Dashs(16), Dashs(12)))
		if ok {
			f.Flush()
		}
	}
	mapLock.Unlock()
}

// resetCompletedJobInfos resets a given job details (only if it has been completed/stopped before) for restarting.
func resetCompletedJobInfos(j *Job) {
	j.pid = 0
	j.iscompleted, j.issuccess = false, false
	j.fetchcount = 0
	j.isstreaming = false
	j.exitcode = -1
	j.errormsg = ""
	j.starttime, j.endtime = time.Time{}, time.Time{}
	(j.result).Reset()
}

// truncateSyntax takes a command syntax as input and returns a shorten version of this syntax
// while taking into account the maximun number of characters.
func truncateSyntax(syntax string, maxlength int) string {
	if utf8.RuneCountInString(syntax) > maxlength {
		r := []rune(syntax)
		syntax = string(r[:maxlength])
	}
	return syntax
}

// formatSize takes the size of a bytes buffer or file in float64 and converts to KB
// then formats it with 0 - 4 digits after the point depending of the value.
func formatSize(size float64) string {
	size = size / 1024
	if size < 10.0 {
		return fmt.Sprintf("%.4f", size)
	} else if size < 100.0 {
		return fmt.Sprintf("%.3f", size)
	} else if size < 1000.0 {
		return fmt.Sprintf("%.2f", size)
	} else if size < 10000.0 {
		return fmt.Sprintf("%.1f", size)
	} else {
		return fmt.Sprintf("%.0f", size)
	}
}

// checkJobsStatusById tells us if one or multiple submitted jobs are either in progress
// or terminated or do not exist. triggered for following request : /jobs/status?id=xx&id=xx
func checkJobsStatusById(w http.ResponseWriter, r *http.Request) {
	// try to setup the response as not buffered data. if succeeds it will be used to flush.
	f, ok := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	// parse all query strings.
	query := r.URL.Query()
	ids, exist := query["id"]
	if !exist || len(ids) == 0 {
		// request does not contains query string id.
		w.WriteHeader(400)
		w.Write([]byte("\n[+] Sorry, the request sent is malformed. Please check details at https://server-ip:8080/"))
		return
	}

	// cleanup user submitted list of jobs ids.
	if ignore := removeDuplicateJobIds(&ids); ignore {
		// no remaining good job id.
		w.WriteHeader(400)
		w.Write([]byte("\n[+] Hello • The request sent does not contain any valid job id remaining after verification.\n"))
		return
	}

	w.WriteHeader(200)
	w.Write([]byte("\n[+] status of the jobs [non-existent or invalid job ids will be ignored, <mem> is memory limit in megabytes] - zoom in/out to fit the screen\n\n"))
	if ok {
		f.Flush()
	}
	// format the display table.
	title := fmt.Sprintf("|%-4s | %-16s | %-5s | %-5s | %-5s | %-7s | %-9s | %-9s | %-5s | %-5s | %-5s | %-7s | %-20s | %-20s | %-20s | %-30s |", "Nb", "Job ID", "PID", "Long", "Done", "Success", "Exit Code", "Data [KB]", "Count", "Mem", "CPU%", "Timeout", "Submitted At [UTC]", "Started At [UTC]", "Ended At [UTC]", "Command Syntax")
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(16), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(9), Dashs(9), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(20), Dashs(20), Dashs(20), Dashs(30)))
	if ok {
		f.Flush()
	}
	// retreive each job status and send.
	i := 0
	var errorsMessages string
	var start, end, sizeFormat string
	mapLock.RLock()
	for _, id := range ids {

		job, exist := globalJobsResults[id]

		if !exist {
			// job id does not exist.
			errorsMessages = errorsMessages + fmt.Sprintf("\n[-] Job ID [%s] does not exist - maybe wrong id or removed from store\n", id)
			continue
		}
		i += 1

		// format time display for zero time values.
		if (job.starttime).IsZero() {
			start = "N/A"
		} else {
			start = (job.starttime).Format("2006-01-02 15:04:05")
		}

		if (job.endtime).IsZero() {
			end = "N/A"
		} else {
			end = (job.endtime).Format("2006-01-02 15:04:05")
		}

		if !job.islong {
			// short job, get memory buffer length.
			sizeFormat = formatSize(float64(job.result.Len()))
		} else if job.islong && job.dump {
			// long job with dumping to file option, use file size.
			fi, err := os.Stat(job.filename)
			if err == nil {
				sizeFormat = formatSize(float64(fi.Size()))
			} else {
				sizeFormat = "N/A"
			}
		} else {
			// long job with dump option.
			sizeFormat = "N/A"
		}
		// stream the added job details to user/client.
		fmt.Fprintln(w, fmt.Sprintf("|%04d | %-16s | %05d | %-5v | %-5v | %-7v | %-9d | %-9s | %-5d | %-5d | %-5d | %-7d | %-20v | %-20v | %-20v | %-30s |", i, job.id, job.pid, job.islong, job.iscompleted, job.issuccess, job.exitcode, sizeFormat, job.fetchcount, job.memlimit, job.cpulimit, job.timeout, (job.submittime).Format("2006-01-02 15:04:05"), start, end, truncateSyntax(job.task, 30)))
		fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(16), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(9), Dashs(9), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(20), Dashs(20), Dashs(20), Dashs(30)))
		if ok {
			f.Flush()
		}
	}
	mapLock.RUnlock()
	// send all errors collected.
	fmt.Fprintln(w, errorsMessages)
	if ok {
		f.Flush()
	}
}

// getAllJobsStatus display all jobs details by sorting (either in ascending or descending) them based on submitted time
// then started time and then ended time. It is triggered for the following request : /jobs/status/?order=asc|desc
func getAllJobsStatus(w http.ResponseWriter, r *http.Request) {
	// try to setup the response as not buffered data. if succeeds it will be used to flush.
	f, ok := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	w.WriteHeader(200)
	mapLock.RLock()
	lg := len(globalJobsResults)
	mapLock.RUnlock()
	if lg == 0 {
		w.Write([]byte("\n[+] There is currently no jobs submitted or available into the results queue to display.\n"))
		return
	}

	w.Write([]byte("\n[+] status of all current jobs [job <done> with <exitcode> -1 means stopped, <mem> is memory limit in megabytes, <long> means long running job] - zoom in/out to fit the screen\n\n"))
	if ok {
		f.Flush()
	}
	// format the display table.
	title := fmt.Sprintf("|%-4s | %-16s | %-5s | %-5s | %-5s | %-7s | %-9s | %-9s | %-5s | %-5s | %-5s | %-7s | %-20s | %-20s | %-20s | %-30s |", "Nb", "Job ID", "PID", "Long", "Done", "Success", "Exit Code", "Data [KB]", "Count", "Mem", "CPU%", "Timeout", "Submitted At [UTC]", "Started At [UTC]", "Ended At [UTC]", "Command Syntax")
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(16), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(9), Dashs(9), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(20), Dashs(20), Dashs(20), Dashs(30)))
	if ok {
		f.Flush()
	}
	// constructs slice of all jobs from the results map.
	var jobs []*Job
	mapLock.RLock()
	for _, job := range globalJobsResults {
		jobs = append(jobs, job)
	}
	mapLock.RUnlock()

	// retrieve odering parameter and sort the slice of jobs based on that.
	order := r.URL.Query().Get("order")
	if order == "desc" {
		// sorting by descending order.
		sort.Slice(jobs, func(i, j int) bool {
			jobi, jobj := jobs[i], jobs[j]
			switch {
			case !jobi.submittime.Equal(jobj.submittime):
				return jobi.submittime.After(jobj.submittime)
			case !jobi.starttime.Equal(jobj.starttime):
				return jobi.starttime.After(jobj.starttime)
			default:
				return jobi.endtime.After(jobj.endtime)
			}
		})

	} else {
		// default by sorting in ascending order.
		sort.Slice(jobs, func(i, j int) bool {
			jobi, jobj := jobs[i], jobs[j]
			switch {
			case !jobi.submittime.Equal(jobj.submittime):
				return jobi.submittime.Before(jobj.submittime)
			case !jobi.starttime.Equal(jobj.starttime):
				return jobi.starttime.Before(jobj.starttime)
			default:
				return jobi.endtime.Before(jobj.endtime)
			}
		})
	}

	// loop over slice of jobs and send each job's status.
	i := 0
	var start, end, sizeFormat string
	for _, job := range jobs {
		i += 1
		// format time display for zero time values.
		if (job.starttime).IsZero() {
			start = "N/A"
		} else {
			start = (job.starttime).Format("2006-01-02 15:04:05")
		}

		if (job.endtime).IsZero() {
			end = "N/A"
		} else {
			end = (job.endtime).Format("2006-01-02 15:04:05")
		}

		if !job.islong {
			// short job, get memory buffer length.
			sizeFormat = formatSize(float64(job.result.Len()))
		} else if job.islong && job.dump {
			// long job with dumping to file option, use file size.
			fi, err := os.Stat(job.filename)
			if err == nil {
				sizeFormat = formatSize(float64(fi.Size()))
			} else {
				sizeFormat = "N/A"
			}
		} else {
			// long job with dump option.
			sizeFormat = "N/A"
		}
		// stream the added job details to user/client.
		fmt.Fprintln(w, fmt.Sprintf("|%04d | %-16s | %05d | %-5v | %-5v | %-7v | %-9d | %-9s | %-5d | %-5d | %-5d | %-7d | %-20v | %-20v | %-20v | %-30s |", i, job.id, job.pid, job.islong, job.iscompleted, job.issuccess, job.exitcode, sizeFormat, job.fetchcount, job.memlimit, job.cpulimit, job.timeout, (job.submittime).Format("2006-01-02 15:04:05"), start, end, truncateSyntax(job.task, 30)))
		fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(16), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(9), Dashs(9), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(20), Dashs(20), Dashs(20), Dashs(30)))
		if ok {
			f.Flush()
		}
	}
}

// getJobsOutputById retreive result output for a given (*job).
func getJobsOutputById(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf8")

	// expect one value for the query - if multiple passed only first will be returned.
	id := r.URL.Query().Get("id")
	// make sure id provided matche - 16 hexa characters.
	if match, _ := regexp.MatchString(`[a-z0-9]{16}`, id); !match {
		w.WriteHeader(400)
		fmt.Fprintln(w, "Hello • The Job ID provided is invalid.")
		return
	}

	// get read lock and verify existence of job into the results map.
	mapLock.RLock()
	job, exist := globalJobsResults[id]
	mapLock.RUnlock()
	if !exist {
		fmt.Fprintln(w, fmt.Sprintf("\n[-] Job ID [%s] does not exist - could have been removed or expired", id))
		return
	}
	// increment number of the job result calls.
	// mapLock.Lock()
	job.fetchcount += 1
	// atomic.AddInt64((*job).getCount, 1)
	// mapLock.Unlock()
	// job present - send the result field data.
	w.WriteHeader(200)
	fmt.Fprintln(w, fmt.Sprintf("Hello • Find below the result of Job ID [%s]", (*job).id))
	fmt.Fprintln(w, (job.result).String())
	return
}

// cleanupMapResults runs every <interval> hours and delete job which got <maxcount>
// times requested or completed for more than <deadtime> hours.
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
				if job.fetchcount > maxcount || (time.Since(job.endtime) > (time.Duration(deadtime) * time.Minute)) {
					// remove job which was terminated since deadtime hours or requested 10 times.
					delete(globalJobsResults, id)
					deletedjobslog.Printf("[%s] [%05d] cleanup routine removed job from the results queue\n", id, job.id)
				}
			}
			mapLock.Unlock()
		}
		// relax to avoid cpu spike into this indefinite loop.
		time.Sleep(100 * time.Millisecond)
	}
}

// handleSignal is a function that process SIGTERM from kill command or CTRL-C or more.
func handleSignal(exit chan struct{}, wg *sync.WaitGroup) {
	log.Println("started goroutine for exit signal handling ...")
	defer wg.Done()
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL,
		syscall.SIGTERM, syscall.SIGHUP, os.Interrupt, os.Kill)

	// block until something comes in.
	signalType := <-sigch
	signal.Stop(sigch)
	fmt.Println("exit command received. exiting...")
	// perform cleanup action before terminating if needed.
	fmt.Println("received signal type: ", signalType)
	// remove PID file
	os.Remove(pidFile)
	// close quit - so jobs monitor groutine will be notified.
	close(exit)
	return
}

// getStatus will read stored PID from the local file and will send a
// non impact signal 0 toward that process to check if it exists (running) or not.
func getDeamonStatus() (err error) {
	// default to 0
	var pid int
	// anonymous function which check final value
	// of pid variable and display deamon status.
	defer func() {
		if pid == 0 {
			fmt.Println("status: not active")
			return
		}
		fmt.Println("status: active - pid", pid)
	}()

	// read the pid from the file.
	pid, err = getPID()
	if err != nil {
		if os.IsNotExist(err) {
			// the file does not exist.
			return nil
		}
		return err
	}

	// case for windows platform - use tasklist command.
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd", "/C", fmt.Sprintf("tasklist | findstr %d", pid)).Output()
		if err != nil {
			fmt.Println(err)
			fmt.Printf("no process with pid %d - removing PID file\n", pid)
			os.Remove(pidFile)
			// set back to 0 so defer function can display inactive status.
			pid = 0
			return err
		}

		// no error - command successfully executed
		// check output lenght if any data inside
		// no data means pid not found - so inactive.
		if len(out[:]) == 0 {
			pid = 0
		}
		// bytes data inside output so return without changing
		// the pid value to have the defer print active.
		return nil
	}

	// try to locate the process.
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	// send dummy signal 0 to that process.
	if err = process.Signal(syscall.Signal(0)); err != nil && err == syscall.ESRCH {
		fmt.Printf("process with pid %d is not running - removing the pid file\n", pid)
		os.Remove(pidFile)
		// set back to 0 so defer function can display inactive status.
		pid = 0
	}
	return nil
}

func restartDeamon() error {
	// read the pid from the file.
	pid, err := getPID()
	if err != nil {
		if os.IsNotExist(err) {
			// the file does not exist. start deamon
			return startDeamon()
		}
		// file may exist. remove it and start
		os.Remove(pidFile)
		return startDeamon()
	}

	// existig pid retreived so remove the file and kill process
	os.Remove(pidFile)
	process, err := os.FindProcess(pid)
	if err != nil {
		// did not find process - may has exited. start
		return startDeamon()
	}

	if err := process.Kill(); err != nil {
		// failed to kill existing process
		fmt.Printf("failed to kill existing process with ID [%v]\n", pid)
		return err
	}
	// succeed to kill existing - so start new.
	return startDeamon()
}

func startDeamon() error {

	// check if the deamon is already running.
	if _, err := os.Stat(pidFile); err == nil {
		fmt.Printf("deamon is running or %s pid file exists. try to restart.\n", pidFile)
		return err
	}

	// launch the program with deamon as argument
	// syscall exec is not the same as fork.
	cmd := exec.Command(os.Args[0], "run")
	// setup the deamon working directory current folder.
	cmd.Dir = "."
	// single file to log output of worker - read by all and write only by the user.
	workerlog, err := os.OpenFile("worker.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("failed to create or open deamon process log file worker.log")
		return err
	}

	cmd.Stdout, cmd.Stderr = workerlog, workerlog

	// start the process asynchronously.
	if err := cmd.Start(); err != nil {
		fmt.Println("failed to start the pseudo deamon process")
		return err
	}

	if err := savePID(cmd.Process.Pid); err != nil {
		// try to kill the process
		if err := cmd.Process.Kill(); err != nil {
			fmt.Printf("failed to kill pseudo deamon process [%d]\n", cmd.Process.Pid)
		}

		return err
	}

	// speudo deamon started
	log.Printf("started deamon with pid [%d] - ppid was [%d]\n", cmd.Process.Pid, os.Getpid())

	// below exit will terminate the group leader (current process)
	// so once you close the terminal - the session will be terminated
	// which will force the pseudo deamon to be terminated as well.
	// that is why it is not a real deamon since it does not survives
	// to its parent lifecycle here.
	// os.Exit(0)
	return nil
}

func stopDeamon() error {

	// read the pid from the file.
	pid, err := getPID()
	if err != nil {
		if os.IsNotExist(err) {
			// the file does not exist.
			fmt.Printf("cannot find pid file %q to load process ID\n", pidFile)
			return nil
		}
		return err
	}

	// try to locate the process.
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	// remove the PID file
	os.Remove(pidFile)

	if err := process.Kill(); err != nil {
		fmt.Printf("failed to kill process with ID [%v]\n", pid)
		return err
	} else {
		fmt.Printf("stopped deamon at pid [%v]\n", pid)
		return nil
	}
}

// setupLoggers ensures the log folder exist and configures all files needed for logging.
func setupLoggers() {
	// build log folder based on current launch day.
	logfolder := fmt.Sprintf("%d%02d%02d.logs", time.Now().Year(), time.Now().Month(), time.Now().Day())
	// use current day log folder to store all logs files. if the folder does not exist, create it.
	info, err := os.Stat(logfolder)
	if errors.Is(err, os.ErrNotExist) {
		// folder does not exist.
		err := os.Mkdir(logfolder, 0755)
		if err != nil {
			// failed to create directory.
			log.Printf("program aborted - failed to create %q logging folder - errmsg: %v", logfolder, err)
			// try to remove PID file.
			os.Remove(pidFile)
			os.Exit(1)
		}
	} else {
		// folder or path already exists but abort if not a directory.
		if !info.IsDir() {
			log.Printf("%q already exists but it is not a folder so check before continue - errmsg: %v\n", logfolder, err)
			// try to remove PID file.
			os.Remove(pidFile)
			os.Exit(0)
		}
	}

	// create the log file for web server requests.
	logfile, err := os.OpenFile(logfolder+string(os.PathSeparator)+"web.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("program aborted - failed to create requests log file - errmsg: %v\n", err)
		// try to remove PID file.
		os.Remove(pidFile)
		os.Exit(1)
	}

	// setup logging format and parameters.
	weblog = log.New(logfile, "", log.LstdFlags|log.Lshortfile)

	// create file to log deleted jobs by cleanupMapResults goroutine.
	deletedjobslogfile, err := os.OpenFile(logfolder+string(os.PathSeparator)+"deleted.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("program aborted - failed to create jobs deletion log file - errmsg: %v\n", err)
		// try to remove PID file.
		os.Remove(pidFile)
		os.Exit(1)
	}

	// setup logging format and parameters.
	deletedjobslog = log.New(deletedjobslogfile, "", log.LstdFlags|log.Lshortfile)

	// create file to log jobs related activities.
	jobslogfile, err := os.OpenFile(logfolder+string(os.PathSeparator)+"jobs.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("program aborted - failed to create jobs processing log file - errmsg: %v\n", err)
		// try to remove PID file.
		os.Remove(pidFile)
		os.Exit(1)
	}

	// setup logging format and parameters.
	jobslog = log.New(jobslogfile, "", log.LstdFlags|log.Lshortfile)
	log.Println("logs folder and all log files successfully created.")
}

// logRequestMiddleware is middleware that logs incoming request details.
func logRequestMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		weblog.Printf("received request - ip: %s - method: %s - url: %s - browser: %s\n", r.RemoteAddr, r.Method, r.URL.Path, r.UserAgent())
		next.ServeHTTP(w, r)
	})
}

// createFolder makes sure that <folderName> is present, and if not creates it.
func createFolder(folderName string) {
	info, err := os.Stat("certs")
	if errors.Is(err, os.ErrNotExist) {
		// path does not exist.
		err := os.Mkdir(folderName, 0755)
		if err != nil {
			log.Printf("failed create %q folder - errmsg : %v\n", folderName, err)
			// try to remove PID file.
			os.Remove(pidFile)
			os.Exit(1)
		}
	} else {
		// path already exists but could be file or directory.
		if !info.IsDir() {
			log.Printf("path %q exists but it is not a folder so please check before continue - errmsg : %v\n", folderName, err)
			// try to remove PID file.
			os.Remove(pidFile)
			os.Exit(0)
		}
	}
}

// generateServerCertificate builds self-signed certificate and rsa-based keys then returns their pem-encoded
// format after has saved them on disk. It aborts the program if any failure during the process.
func generateServerCertificate() ([]byte, []byte) {
	log.Println("generating new self-signed server's certificate and private key")
	// ensure the presence of "certs" folder to store server's certs & key.
	createFolder("certs")
	// https://pkg.go.dev/crypto/x509#Certificate
	serverCerts := &x509.Certificate{
		SignatureAlgorithm: x509.SHA256WithRSA,
		PublicKeyAlgorithm: x509.RSA,
		// generate a random serial number
		SerialNumber: big.NewInt(2021),
		// define the PKIX (Internet Public Key Infrastructure Using X.509).
		// fill each field with the right information based on your context.
		Subject: pkix.Name{
			Organization:  []string{"My Company"},
			Country:       []string{"My Country"},
			Province:      []string{"My City"},
			Locality:      []string{"My Locality"},
			StreetAddress: []string{"My Street Address"},
			PostalCode:    []string{"00000"},
			CommonName:    "localhost",
		},

		NotBefore: time.Now(),
		// make it valid for 1 year.
		NotAfter: time.Now().AddDate(1, 0, 0),
		// self-signed certificate.
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		// enter the right email address here.
		EmailAddresses: []string{"my-email@mydomainname"},
		// in upcoming release, server could start with user-defined ip or dnsname so these
		// below two fields will be dynamically filled. For now, this server runs locally.
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:    []string{"localhost"},
	}

	// generate a public & private key for the certificate.
	serverPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Printf("failed to generate server private key - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(pidFile)
		os.Exit(1)
	}

	log.Println("successfully created rsa-based key for server certificate.")

	// pem encode the private key.
	serverPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(serverPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(serverPrivKey),
	})

	if err != nil {
		log.Printf("failed to pem encode server private key - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(pidFile)
		os.Exit(2)
	}

	// dump CA private key into a file.
	if err := os.WriteFile(serverPrivKeyPath, serverPrivKeyPEM.Bytes(), 0644); err != nil {
		log.Printf("failed to save on disk the server private key - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(pidFile)
		os.Exit(1)
	}

	log.Println("successfully pem encoded and saved server's private key.")

	// create the server certificate. https://pkg.go.dev/crypto/x509#CreateCertificate
	serverCertsBytes, err := x509.CreateCertificate(rand.Reader, serverCerts, serverCerts, &serverPrivKey.PublicKey, serverPrivKey)
	if err != nil {
		log.Printf("failed to create server certificate - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(pidFile)
		os.Exit(1)
	}

	log.Println("successfully created server certificate.")

	// pem encode the certificate.
	serverCertsPEM := new(bytes.Buffer)
	err = pem.Encode(serverCertsPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertsBytes,
	})

	if err != nil {
		log.Printf("failed to pem encode server certificate - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(pidFile)
		os.Exit(2)
	}

	// dump certificate into a file.
	if err := os.WriteFile(serverCertsPath, serverCertsPEM.Bytes(), 0644); err != nil {
		log.Printf("failed to save on disk the server certificate - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(pidFile)
		os.Exit(1)
	}

	log.Println("successfully pem encoded and saved server's certificate.")

	return serverCertsPEM.Bytes(), serverPrivKeyPEM.Bytes()
}

func startWebServer(exit <-chan struct{}) error {

	// we try to load server certificate and key from disk.
	serverTLSCerts, err := tls.LoadX509KeyPair(serverCertsPath, serverPrivKeyPath)
	if err != nil {
		// if it fails for some reasons - we rebuild new certificate and key.
		log.Printf("failed to load server's certificate and key from disk - errmsg : %v", err)
		// rebuild self-signed certs and constructs server TLS certificate.
		serverTLSCerts, err = tls.X509KeyPair(generateServerCertificate())
		if err != nil {
			log.Printf("failed to load server's pem-encoded certificate and key - errmsg : %v\n", err)
			// try to remove PID file.
			os.Remove(pidFile)
			os.Exit(1)
		}
	}

	// build server TLS configurations - https://pkg.go.dev/crypto/tls#Config
	tlsConfig := &tls.Config{
		// handshake with minimum TLS 1.2. MaxVersion is 1.3.
		MinVersion: tls.VersionTLS12,
		// server certificates (key with self-signed certs).
		Certificates: []tls.Certificate{serverTLSCerts},
		// elliptic curves that will be used in an ECDHE handshake, in preference order.
		CurvePreferences: []tls.CurveID{tls.CurveP521, tls.X25519, tls.CurveP256},
		// CipherSuites is a list of enabled TLS 1.0–1.2 cipher suites. The order of
		// the list is ignored. Note that TLS 1.3 ciphersuites are not configurable.
		CipherSuites: []uint16{
			// TLS v1.2 - ECDSA-based keys cipher suites.
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			// TLS v1.2 - RSA-based keys cipher suites.
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			// TLS v1.3 - some strong cipher suites.
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
		},
	}

	// start the web server.
	router := http.NewServeMux()
	// default request - list how to details.
	router.HandleFunc("/", webHelp)
	// expected request : /worker/v1/cmd/execute?cmd=<task-syntax>&timeout=<value>
	router.HandleFunc("/worker/v1/cmd/execute", instantCommandExecutor)
	// expected request : /worker/v1/jobs/short/schedule?cmd=<task-syntax>
	router.HandleFunc("/worker/v1/jobs/short/schedule", scheduleShortRunningJobs)
	// expected request : /worker/v1/jobs/x/status/check?id=<jobid>
	router.HandleFunc("/worker/v1/jobs/x/status/check", checkJobsStatusById)
	// expected format : /worker/v1/jobs/short/output/fetch?id=<jobid>
	router.HandleFunc("/worker/v1/jobs/short/output/fetch", getJobsOutputById)
	// expected request : /worker/v1/jobs/x/status/check/all?order=asc|desc
	router.HandleFunc("/worker/v1/jobs/x/status/check/all", getAllJobsStatus)
	// expected request : /worker/v1/jobs/x/stop?id=<jobid>&id=<jobid>
	router.HandleFunc("/worker/v1/jobs/x/stop", stopJobsById)
	// expected request : /worker/v1/jobs/x/stop/all
	router.HandleFunc("/worker/v1/jobs/x/stop/all", stopAllJobs)
	// expected request : /worker/v1/jobs/x/restart?id=<jobid>&id=<jobid>
	router.HandleFunc("/worker/v1/jobs/x/restart", restartJobsById)
	// expected request : /worker/v1/jobs/x/restart/all
	router.HandleFunc("/worker/v1/jobs/x/restart/all", restartAllJobs)
	// live streaming a long running job output: /worker/v1/jobs/long/output/stream?id=<jobid>
	router.HandleFunc("/worker/v1/jobs/long/output/stream", streamJobsOutputById)
	// schedule a long running job with output streaming capability.
	// /worker/v1/jobs/long/stream/schedule?cmd=<task>&cmd=<task>&timeout=<value>&save=true|false
	router.HandleFunc("/worker/v1/jobs/long/stream/schedule", scheduleLongJobsWithStreaming)
	// schedule a long running job with only streaming output to disk file.
	// /worker/v1/jobs/long/dump/schedule?cmd=<task>&cmd=<task>&timeout=<value>
	// router.HandleFunc("/worker/v1/jobs/long/dump/schedule", scheduleLongJobsWithDumping)

	webserver := &http.Server{
		Addr:         address,
		Handler:      logRequestMiddleware(router),
		ErrorLog:     weblog,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLSConfig:    tlsConfig,
	}

	// goroutine in charge of shutting down the server when triggered.
	go func() {
		log.Println("started goroutine to shutdown web server ...")
		// wait until close or something comes in.
		<-exit
		log.Printf("shutting down the web server ... max waiting for 60 secs.")
		ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)

		if err := webserver.Shutdown(ctx); err != nil {
			// error due to closing listeners, or context timeout.
			log.Printf("failed to shutdown gracefully the web server - errmsg: %v", err)
			if err == context.DeadlineExceeded {
				log.Printf("the web server did not shutdown before 45 secs deadline.")
			} else {
				log.Printf("an error occured when closing underlying listeners.")
			}

			return
		}

		// err = nil - successfully shutdown the server.
		log.Println("the web server was successfully shutdown down.")

	}()

	// make listen on all interfaces - helps on container binding
	// if shutdown error will be http.ErrServerClosed.
	log.Printf("starting https only web server on %s ...\n", address)
	weblog.Printf("starting web server at %s\n", address)
	if err := webserver.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		log.Fatalf("failed to start the web server on %s - errmsg: %v\n", address, err)
		return err
	}

	return nil
}

func runDeamon() error {
	user, err := user.Current()
	if err != nil {
		log.Printf("failed to retrieve owner name of this worker process - errmsg: %v", err)
		// return err
	}

	log.Printf("started deamon - pid [%d] - user [%s] - ppid [%d]\n", os.Getpid(), user.Username, os.Getppid())
	// silently remove pid file if deamon exit.
	defer func() {
		os.Remove(pidFile)
		os.Exit(0)
	}()
	// to make sure all goroutines exit before leaving the program.
	wg := &sync.WaitGroup{}
	// channel to instruct jobs monitor to exit.
	exit := make(chan struct{}, 1)
	// background handler for interruption signals.
	wg.Add(1)
	go handleSignal(exit, wg)
	// execute the init routine.
	initializer()
	// background jobs map cleaner.
	wg.Add(1)
	go cleanupMapResults(interval, maxcount, maxage, exit, wg)
	// start the jobs queue monitor
	wg.Add(1)
	go jobsMonitor(exit, wg)
	// setup loggers
	setupLoggers()

	// run http web server and block until server exits (shutdown).
	err = startWebServer(exit)
	// after web server stopped - block until all goroutines exit as well.
	wg.Wait()
	log.Printf("successfully stopped all goroutines from worker - pid [%d] - user [%s] - ppid [%d]\n", os.Getpid(), user.Username, os.Getppid())
	return err
}

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	if len(os.Args) != 2 {
		// returned the program name back since someone could have
		// compiled with another output name. Then gracefully exit.
		fmt.Printf("Usage: %s [start|stop|restart|status] \n", os.Args[0])
		os.Exit(0)
	}

	var err error
	option := strings.ToLower(os.Args[1])

	switch option {
	case "run":
		err = runDeamon()
	case "start":
		err = startDeamon()
	case "stop":
		err = stopDeamon()
	case "status":
		err = getDeamonStatus()
	case "restart":
		err = restartDeamon()
	case "help":
		displayHelp()
	case "version":
		displayVersion()
	default:
		// not implemented command.
		fmt.Printf("unknown command: %v\n", os.Args[1])
		fmt.Printf("Usage: %s [start|stop|restart|status|help|version] \n", os.Args[0])
	}

	if err != nil {
		fmt.Println(option, "error:", err)
		os.Exit(1)
	}

	// below exit will terminate the group leader (current process)
	// so once you close the terminal - the session will be terminated
	// which will force the pseudo deamon to be terminated as well.
	// that is why it is not a real deamon since it does not survives
	// to its parent lifecycle here. So, keep the launcher terminal open.
	os.Exit(0)
}

func displayVersion() {
	fmt.Printf("\n%s\n", version)
	os.Exit(0)
}

func displayHelp() {
	fmt.Printf("\n%s\n", help)
	os.Exit(0)
}
