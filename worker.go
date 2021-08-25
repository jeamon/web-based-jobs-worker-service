package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
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

const webserver = "0.0.0.0:8080"

// waiting time before program exit at failure.
const waitingTime = 3

// cleanup routine default values.
const maxcount = 10
const interval = 12
const maxage = 24

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

// Job has unique id with task as command to execute
// result field will save output of the execution
// a status (if true - means finished otherwise in progress)
// begin and ended fields to track start & finish timestamp.
type Job struct {
	id          string
	pid         int
	task        string
	memlimit    int
	cpulimit    int
	timeout     int
	iscompleted bool
	issuccess   bool
	exitcode    int
	errormsg    string
	fetchcount  int
	stop        chan struct{}
	result      bytes.Buffer
	submittime  time.Time
	starttime   time.Time
	endtime     time.Time
}

// jobs allocator buffer (max jobs to into batch).
const maxJobs = 10

// buffered (maxJobs) channel to hold submitted (*job).
var globalJobsQueue chan Job

// store all submitted jobs after processed with job id as key.
var globalJobsResults map[string]*Job
var mapLock *sync.RWMutex

func initializer() {
	mapLock = &sync.RWMutex{}
	globalJobsQueue = make(chan Job, maxJobs)
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
	weblog.Printf("received request - %s\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	fmt.Fprintf(w, help)

	return
}

// instantCommandExecutor is a function that execute a single passed command and send the result.
// examples: curl localhost:8080/execute?cmd=dir+/B
// curl localhost:8080/execute?cmd=mkdir+jerome
// curl localhost:8080/execute?cmd=rmdir+jerome
// replacing string. use + after ? AND use %20 before ?
func instantCommandExecutor(w http.ResponseWriter, r *http.Request) {
	weblog.Printf("received request - [cmd] - %s\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	msg := []byte("\n[+] Hello • failed to execute the command passed.\n")

	// expect one value for the query - if multiple passed only first will be returned.
	cmd := r.URL.Query().Get("cmd")

	// execute listing command for windows.
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/C", cmd).Output(); err == nil {
			w.WriteHeader(200)
			w.Write(append([]byte("\n[+] Hello • find below output of the command.\n\n"), out[:]...))
		} else {
			w.WriteHeader(503)
			w.Write(append(msg, []byte(err.Error())...))
		}
		return
	}

	// execute passed command for linux-based platforms.
	if out, err := exec.Command(shell, "-c", cmd).Output(); err == nil {
		w.WriteHeader(200)
		w.Write(append([]byte("\n[+] Hello • find below output of the command.\n\n"), out[:]...))
	} else {
		w.WriteHeader(503)
		w.Write(append(msg, []byte(err.Error())...))
	}
	w.WriteHeader(400)
	return
}

// build a string made of dash symbol - used to display table.
func Dashs(count int) string {
	return strings.Repeat("-", count)
}

// executeJob takes a job structure and will execute the task and add the result to the global results bus.
func executeJob(job Job, ctx context.Context) {
	job.starttime = time.Now().UTC()

	jobctx := ctx
	if job.timeout > 0 {
		// job has timeout option provided into minutes.
		jobctx, _ = context.WithTimeout(ctx, time.Duration(job.timeout)*time.Minute)
	}

	jobslog.Printf("started the processing of job with id [%s]\n", job.id)
	var cmd *exec.Cmd

	// command syntax for windows platform.
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(jobctx, "cmd", "/C", job.task)
	} else {
		// syntax for linux-based platforms.
		cmd = exec.CommandContext(jobctx, shell, "-c", job.task)
	}

	// set the job result field for combined output.
	cmd.Stdout, cmd.Stderr = &job.result, &job.result

	// asynchronous start
	if err := cmd.Start(); err != nil {
		// no need to continue - add job stats to map.
		job.endtime = time.Now().UTC()
		jobslog.Printf("failed to start job with id [%s] - errmsg: %v\n", job.id, err)
		job.issuccess = false
		job.errormsg = err.Error()
		job.iscompleted = true
		mapLock.Lock()
		globalJobsResults[job.id] = &job
		mapLock.Unlock()
		return
	}
	// job process started.
	job.pid = cmd.Process.Pid

	// var err error
	done := make(chan error)

	go func() {
		done <- cmd.Wait()
	}()

	var err error
	// block on select until one case hits.
	select {

	case <-jobctx.Done():

		switch jobctx.Err() {

		case context.DeadlineExceeded:
			// timeout reached - so try to kill the job process.
			jobslog.Printf("timeout reached - failed to complete processing of job with id [%s]\n", job.id)
		case context.Canceled:
			// context cancellation triggered.
			jobslog.Printf("job execution cancelled - failed to complete processing of job with id [%s]\n", job.id)
		}

		// kill the process and exit from this function.
		if perr := cmd.Process.Kill(); perr != nil {
			jobslog.Printf("failed to kill process id [%d] of job with id [%s] - errmsg: %v\n", cmd.Process.Pid, job.id, perr)
		} else {
			jobslog.Printf("succeeded to kill process id [%d] of job with id [%s]\n", cmd.Process.Pid, job.id)
		}
		// leave the select loop.
		break
	case err = <-done:
		// task completed before timeout. exit from select loop.
		break
	}

	job.endtime = time.Now().UTC()
	job.iscompleted = true

	if err != nil {
		// timeout not reached - but an error occured during execution
		jobslog.Printf("error occured during execution - failed to complete job with id [%s] - errmsg: %v\n", job.id, err)
		job.issuccess = false
		job.errormsg = err.Error()
		// lets get the exit code.
		if exitError, ok := err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			job.exitcode = ws.ExitStatus()
		}
	}

	if err == nil {
		jobslog.Printf("completed processing with success of job with id [%s]\n", job.id)
		job.issuccess = true
		// success, exitCode should be 0
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		job.exitcode = ws.ExitStatus()
	}

	// get write lock and add the final state of the job to results map.
	mapLock.Lock()
	globalJobsResults[job.id] = &job
	mapLock.Unlock()
	jobslog.Printf("save the full result of the execution of job with id [%s]\n", job.id)
}

// jobsMonitor watches the global jobs queue and spin up a separate executor to handle the task.
func jobsMonitor(exit <-chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	// indefinite loop on the jobs channel.
	for {
		select {
		case job := <-globalJobsQueue:
			// go execute(job, ctx)
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

// handleJobsRequests handles user jobs submission request and build the jobs structure for further processing.
func handleJobsRequests(w http.ResponseWriter, r *http.Request) {

	weblog.Printf("received request - [job] - %s\n", r.URL.Path)
	// try to setup the response as not buffered data.
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	//	msg := []byte("\n[+] Hello • failed to execute the command passed.\n")
	help := []byte("\n[+] Hello • The request sent is malformed • The expected format is :\n http://server-ip:8080/jobs?cmd=xxx&cmd=xxx&mem=megabitsValue&cpu=percentageValue\n")

	// parse all query strings.
	query := r.URL.Query()
	cmds, exist := query["cmd"]
	if !exist || len(cmds) == 0 {
		// request does not contains query string cmd.
		w.WriteHeader(400)
		w.Write(help)
		return
	}
	// default memory (megabytes) and cpu (percentage) limit values.
	memlimit := 100
	cpulimit := 10
	// extract only first value of mem and cpu query string.
	if m, err := strconv.Atoi(query.Get("mem")); err == nil && m > 0 {
		memlimit = m
	}

	if c, err := strconv.Atoi(query.Get("cpu")); err == nil && c > 0 {
		cpulimit = c
	}
	w.WriteHeader(200)
	w.Write([]byte("\n[+] Hello • Please find below the details of your jobs request :\n\n"))

	// format the display table.
	title := fmt.Sprintf("|%-4s | %-18s | %-14s | %-10s | %-38s | %-20s |", "Nb", "Job ID", "Memory [MB]", "CPU [%]", "Submitted", "Command Syntax")
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(14), Dashs(10), Dashs(38), Dashs(20)))

	// build each job per command with resources limit values.
	for i, cmd := range cmds {
		job := Job{
			id:          generateID(),
			pid:         0,
			task:        cmd,
			iscompleted: true,
			issuccess:   false,
			exitcode:    -1,
			errormsg:    "",
			fetchcount:  0,
			memlimit:    memlimit,
			cpulimit:    cpulimit,
			submittime:  time.Now().UTC(),
			starttime:   time.Time{},
			endtime:     time.Time{},
		}
		// store this job for further processing.
		globalJobsQueue <- job
		jobslog.Printf("added new job with id [%s] to the queue\n", job.id)
		// stream the added job details to user/client.
		fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-14d | %-10d | %-38v | %-20s |", i, job.id, job.memlimit, job.cpulimit, job.submittime, job.task))
		fmt.Fprintln(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+", Dashs(4), Dashs(18), Dashs(14), Dashs(10), Dashs(38), Dashs(20)))
	}
}

// checkJobsStatusById tells us if one or multiple submitted jobs are either in progress
// or terminated or do not exist. triggered for following request : /jobs/status?id=xx&id=xx
func checkJobsStatusById(w http.ResponseWriter, r *http.Request) {
	weblog.Printf("received request - [job] - %s\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")

	// parse all query strings.
	query := r.URL.Query()
	ids, exist := query["id"]
	if !exist || len(ids) == 0 {
		// request does not contains query string id.
		w.WriteHeader(400)
		w.Write([]byte("\n[+] Hello • The request sent is malformed • The expected format is :\n http://server-ip:8080/jobs/status?id=xx&id=xx\n"))
		return
	}
	w.WriteHeader(200)
	w.Write([]byte("\n[+] status of the jobs [non-existent will be ignored] - zoom in to fit the screen\n\n"))

	// format the display table.
	title := fmt.Sprintf("|%-4s | %-18s | %-6s | %-10s | %-12s | %-10s | %-14s | %-10s | %-38s | %-38s | %-38s | %-20s |", "Nb", "Job ID", "Done", "Success", "Exit Code", "Count", "Memory [MB]", "CPU [%]", "Submitted At", "Started At", "Ended At", "Command Syntax")
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(6), Dashs(10), Dashs(12), Dashs(10), Dashs(14), Dashs(10), Dashs(38), Dashs(38), Dashs(38), Dashs(20)))

	// retreive each job status and send.
	i := 0
	mapLock.RLock()
	for _, id := range ids {

		if match, _ := regexp.MatchString(`[a-z0-9]{16}`, id); !match {
			fmt.Fprintln(w, fmt.Sprintf("\n[-] Job ID [%s] is invalid - please make sure to provide the right ID", id))
			continue
		}

		job, exist := globalJobsResults[id]

		if !exist {
			weblog.Printf("status for job with id [%s] does not exist\n", id)
			continue
		}
		i += 1
		// stream the added job details to user/client.
		fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-6v | %-10v | %-12d | %-10d | %-14d | %-10d | %-38v | %-38v | %-38v | %-20s |", i, job.id, job.iscompleted, job.issuccess, job.exitcode, job.fetchcount, job.memlimit, job.cpulimit, job.submittime, job.starttime, job.endtime, job.task))
		fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(6), Dashs(10), Dashs(12), Dashs(10), Dashs(14), Dashs(10), Dashs(38), Dashs(38), Dashs(38), Dashs(20)))
	}
	mapLock.RUnlock()
}

// getAllJobsStatus display all submitted jobs details
// triggered for following request : /jobs/status/
func getAllJobsStatus(w http.ResponseWriter, r *http.Request) {
	weblog.Printf("received request - [job] - %s\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	w.WriteHeader(200)
	w.Write([]byte("\n[+] status of all current jobs - zoom in to fit the screen\n\n"))

	// format the display table.
	title := fmt.Sprintf("|%-4s | %-18s | %-6s | %-10s | %-12s | %-10s | %-14s | %-10s | %-38s | %-38s | %-38s | %-20s |", "Nb", "Job ID", "Done", "Success", "Exit Code", "Count", "Memory [MB]", "CPU [%]", "Submitted At", "Started At", "Ended At", "Command Syntax")
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(6), Dashs(10), Dashs(12), Dashs(10), Dashs(14), Dashs(10), Dashs(38), Dashs(38), Dashs(38), Dashs(20)))

	// retreive each job status and send.
	i := 0
	mapLock.RLock()
	for _, job := range globalJobsResults {
		i += 1
		// stream the added job details to user/client.
		fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-6v | %-10v | %-12d | %-10d | %-14d | %-10d | %-38v | %-38v | %-38v | %-20s |", i, job.id, job.iscompleted, job.issuccess, job.exitcode, job.fetchcount, job.memlimit, job.cpulimit, job.submittime, job.starttime, job.endtime, job.task))
		fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(6), Dashs(10), Dashs(12), Dashs(10), Dashs(14), Dashs(10), Dashs(38), Dashs(38), Dashs(38), Dashs(20)))
	}
	mapLock.RUnlock()
}

// getJobsResultsById retreive result output for a given (*job).
func getJobsResultsById(w http.ResponseWriter, r *http.Request) {
	weblog.Printf("received request - [job] - %s\n", r.URL.Path)
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
	(*job).fetchcount += 1
	// atomic.AddInt64((*job).getCount, 1)
	// mapLock.Unlock()
	// job present - send the result field data.
	w.WriteHeader(200)
	fmt.Fprintln(w, fmt.Sprintf("Hello • Find below the result of Job ID [%s]", (*job).id))
	fmt.Fprintln(w, ((*job).result).String())
	return
}

// cleanupMapResults runs every <interval> hours and delete job which got <maxcount>
// times requested or completed for more than <deadtime> hours.
func cleanupMapResults(interval int, maxcount int, deadtime int, exit <-chan struct{}) {

	for {
		select {
		case <-exit:
			// worker to stop - so leave this goroutine.
			return
		case <-time.After(time.Duration(interval) * time.Hour):
			// waiting time passed so lock the map and loop over each (*job).
			mapLock.Lock()
			for id, job := range globalJobsResults {
				if (*job).fetchcount > maxcount || (time.Since((*job).endtime) > (time.Duration(deadtime) * time.Minute)) {
					// remove job which was terminated since deadtime hours or requested 10 times.
					delete(globalJobsResults, id)
					deletedjobslog.Printf("removed Job ID [%s] from the results queue\n", id)
				}
			}
			mapLock.Unlock()
		}
		// relax to avoid cpu spike into this indefinite loop.
		time.Sleep(100 * time.Millisecond)
	}
}

// processSignal is a function that process SIGTERM from kill command or CTRL-C or more.
func handleSignal(exit chan struct{}) {

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
	// os.Exit(0)
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
	if err = process.Signal(syscall.Signal(0)); err != nil {
		fmt.Printf("no process with pid %d - removing PID file\n", pid)
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
	// setup the deamon working directory to root avoid conflicting.
	// cmd.Dir = "/"

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
	fmt.Printf("started deamon with pid [%d] - ppid was [%d]\n", cmd.Process.Pid, os.Getpid())

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

// setupLoggers create a log folder and configure all files needed for logging.
func setupLoggers() {
	starttime := time.Now() //== get current launch time
	logtime := fmt.Sprintf("%d%02d%02d-%02d%02d%02d", starttime.Year(), starttime.Month(), starttime.Day(), starttime.Hour(), starttime.Minute(), starttime.Second())

	// create dedicated log folder for each launch of the program.
	logfolder := fmt.Sprintf("%s-log", logtime)
	if err := os.Mkdir(logfolder, 0755); err != nil {
		fmt.Printf(" [-] Program aborted. failed to create the dedicated log folder - Errmsg: %v", err)
		time.Sleep(waitingTime * time.Second)
		os.Exit(1)
	}

	// create the log file for web server requests.
	logfile, err := os.OpenFile(logfolder+string(os.PathSeparator)+"web.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Printf(" [*] Program aborted // failed to create requests log file - errmsg // ", err)
		time.Sleep(3 * time.Second)
		os.Exit(1)
	}

	// setup logging format and parameters.
	weblog = log.New(logfile, "[ INFOS ] ", log.LstdFlags|log.Lshortfile)

	// create file to log deleted jobs by cleanupMapResults goroutine.
	deletedjobslogfile, err := os.OpenFile(logfolder+string(os.PathSeparator)+"deleted.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Printf(" [*] Program aborted // failed to create jobs deletion log file - errmsg // ", err)
		time.Sleep(3 * time.Second)
		os.Exit(1)
	}

	// setup logging format and parameters.
	deletedjobslog = log.New(deletedjobslogfile, "[ INFOS ] ", log.LstdFlags|log.Lshortfile)

	// create file to log jobs related activities.
	jobslogfile, err := os.OpenFile(logfolder+string(os.PathSeparator)+"jobs.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Printf(" [*] Program aborted // failed to create jobs processing log file - errmsg // ", err)
		time.Sleep(3 * time.Second)
		os.Exit(1)
	}

	// setup logging format and parameters.
	jobslog = log.New(jobslogfile, "[ INFOS ] ", log.LstdFlags|log.Lshortfile)
}

func startWebServer() error {
	// start the web server.
	mux := http.NewServeMux()
	// default request - list how to details.
	mux.HandleFunc("/", webHelp)
	// expected query string : /execute?cmd=xxxxx
	mux.HandleFunc("/execute", instantCommandExecutor)
	// expected query string : /jobs?cmd=xxxxx
	mux.HandleFunc("/jobs", handleJobsRequests)
	// expected query string : /jobs/status?id=xxxxx
	mux.HandleFunc("/jobs/status", checkJobsStatusById)
	// expected query string : /jobs/results?id=xxxxx
	mux.HandleFunc("/jobs/results", getJobsResultsById)
	// expected URI : /jobs/status/ or /jobs/stats
	mux.HandleFunc("/jobs/status/", getAllJobsStatus)
	mux.HandleFunc("/jobs/stats/", getAllJobsStatus)
	weblog.Printf("started web server at %s\n", webserver)
	// make listen on all interfaces - helps on container binding
	// tun http web server into a separate goroutine.
	err := http.ListenAndServe(webserver, mux)
	weblog.Printf("web server failed to serve - errmsg : %v\n", err)
	return err
}

func runDeamon() error {
	// silently remove pid file if deamon exit.
	defer func() {
		os.Remove(pidFile)
		os.Exit(0)
	}()
	// channel to instruct jobs monitor to exit.
	exit := make(chan struct{})
	// background handler for interruption signals.
	go handleSignal(exit)
	// execute the init routine.
	initializer()
	// background jobs map cleaner.
	go cleanupMapResults(interval, maxcount, maxage, exit)
	// start the jobs queue monitor
	go jobsMonitor(exit)
	// setup loggers
	setupLoggers()

	user, err := user.Current()
	if err != nil {
		weblog.Println(err.Error())
		return err
	}
	weblog.Printf("starting deamon - pid [%d] - with user [%s] - parent pid [%d]\n", os.Getpid(), user.Username, os.Getppid())

	// change working directory to user home directory.
	home, _ := os.UserHomeDir()
	os.Chdir(home)

	// run http web server
	err = startWebServer()
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
