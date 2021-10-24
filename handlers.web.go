package main

// @handlers.web.go contains functions that directly handle each request coming from a web browser.

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// webV1Help sends the documentation for the v1 web routes.
func webV1Help(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	fmt.Fprintf(w, webv1docs)
	return
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
		fmt.Fprintf(w, "\n[+] Sorry, the request submitted is malformed. To view the documentation, go to https://"+Config.HttpsServerHost+":"+Config.HttpsServerPort+"/worker/web/v1/docs")
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
		cmd = exec.CommandContext(ctx, Config.DefaultLinuxShell, "-c", task)
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
	id := generateID(8)
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

// checkJobsStatusById display a summary table of the status of one or multiple jobs based
// on their ids. GET /worker/web/v1/jobs/x/status/check?id=<jobid>&id=<jobid>.
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
	// send the table headers.
	fmt.Fprintf(w, formatJobsStatusTableHeaders())
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
		// stream this job status as a row.
		fmt.Fprintf(w, job.formatStatusAsTableRow(i, start, end, sizeFormat))
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
// then started time and then ended time. GET /worker/web/v1/jobs/x/status/check/all?order=asc|desc.
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
	// send the table headers.
	fmt.Fprintf(w, formatJobsStatusTableHeaders())
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
		// stream this job status as a row.
		fmt.Fprintf(w, job.formatStatusAsTableRow(i, start, end, sizeFormat))
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
	job.lock.Lock()
	job.fetchcount += 1
	job.lock.Unlock()
	// job present - send the result field data.
	w.WriteHeader(200)
	fmt.Fprintln(w, fmt.Sprintf("Hello • Find below the result of Job ID [%s]", job.id))
	fmt.Fprintln(w, (job.result).String())
	return
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
		fmt.Fprintf(w, "\n[+] Sorry, the request submitted is malformed. To view the documentation, go to https://"+Config.HttpsServerHost+":"+Config.HttpsServerPort+"/worker/web/v1/docs")
		return
	}
	// default memory (megabytes) and cpu (percentage) limit values.
	memlimit := Config.MemoryLimitDefaultMegaBytes
	cpulimit := Config.CpuLimitDefaultPercentage
	timeout := Config.ShortJobTimeout
	// extract only first value of mem and cpu query string. they cannot be greater than maximum values.
	if m, err := strconv.Atoi(query.Get("mem")); err == nil && m > 0 && m <= Config.MemoryLimitMaxMegaBytes {
		memlimit = m
	}

	if c, err := strconv.Atoi(query.Get("cpu")); err == nil && c > 0 && c <= Config.CpuLimitMaxPercentage {
		cpulimit = c
	}
	// retreive timeout parameter value and consider it if higher than 0.
	if t, err := strconv.Atoi(query.Get("timeout")); err == nil && t > 0 && t <= Config.ShortJobTimeout {
		timeout = t
	}
	w.WriteHeader(200)
	w.Write([]byte("\n[+] find below some details of the jobs submitted\n\n"))

	// send the table headers.
	fmt.Fprintf(w, formatJobsScheduledTableHeaders())
	if ok {
		f.Flush()
	}
	// build each job per command with resources limit values.
	for i, cmd := range cmds {
		job := &Job{
			id:          generateID(8),
			pid:         0,
			task:        cmd,
			islong:      false,
			stream:      false,
			dump:        false,
			iscompleted: false,
			issuccess:   false,
			exitcode:    -1,
			errormsg:    "",
			fetchcount:  0,
			stop:        make(chan struct{}, 1),
			lock:        &sync.RWMutex{},
			result:      new(bytes.Buffer),
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
		// stream this scheduled job details as a row.
		fmt.Fprintf(w, job.formatScheduledInfosAsTableRow(i+1))
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
		fmt.Fprintf(w, "\n[+] Sorry, the request submitted is malformed. To view the documentation, go to https://"+Config.HttpsServerHost+":"+Config.HttpsServerPort+"/worker/web/v1/docs")
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
		fmt.Fprintf(w, "\n[+] Sorry, the request submitted is malformed. To view the documentation, go to https://"+Config.HttpsServerHost+":"+Config.HttpsServerPort+"/worker/web/v1/docs")
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
			job.resetCompletedJobInfos()
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
	// send the table headers.
	fmt.Fprintf(w, formatJobsRestartTableHeaders())
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
			fmt.Fprintf(w, job.formatRestartInfosAsTableRow(i, "yes", "yes", "n/a"))
		} else {
			// job already completed (not running). so reset and start it by adding to jobs queue in parallel.
			job.resetCompletedJobInfos()
			globalJobsQueue <- job
			jobslog.Printf("[%s] [%05d] restart requested for the completed job. reset the details and scheduled job onto processing queue\n", job.id, job.pid)
			fmt.Fprintf(w, job.formatRestartInfosAsTableRow(i, "no", "n/a", "yes"))
		}
		if ok {
			f.Flush()
		}
	}
	mapLock.Unlock()
}

// downloadJobsOutputById check if id provided belong to a long job which has dump option enabled.
// then loads from disk its output file and for downloading. In case the id belongs to a short job,
// it is ignored because by design short job output is saved into memory buffer. This means if a short
// job has been deleted due to max age or by request, its output dump is just a backup and you will
// need to manually pull it from the server (reach out to the backend administrator for that purpose).
// GET /worker/web/v1/jobs/x/output/download?id=<jobid>
func downloadJobsOutputById(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("Content-Type", "application/octet-stream")
	// expect one value for the query - if multiple passed only first will be returned.
	id := r.URL.Query().Get("id")
	// make sure id provided matche - 16 hexa characters.
	if match, _ := regexp.MatchString(`[a-z0-9]{16}`, id); !match {
		w.Header().Set("Content-Type", "text/plain; charset=utf8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, fmt.Sprintf("Sorry, the Job ID [%s] provided is invalid.", id))
		return
	}

	// get read lock and verify existence of job into the results map.
	mapLock.RLock()
	job, exist := globalJobsResults[id]
	mapLock.RUnlock()
	if !exist {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, fmt.Sprintf("\nSorry, this job ID [%s] does not exist. could have been removed or expired. check into archived outputs folder.", id))
		return
	}

	if !job.islong {
		// short job has output in buffer memory.
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, fmt.Sprintf("\nSorry, this job ID [%s] belongs to a short job. Fetch its output directly from its buffer memory.", id))
		return
	}

	if !job.dump {
		// long job without dump option, so there is no file to load.
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, fmt.Sprintf("\nSorry, this job ID [%s] belongs to a long job without dump option. There is no file to download.", id))
		return
	}

	// get the output file  path to download.
	var outputFilePath string
	if len(job.filename) != 0 {
		outputFilePath = job.filename
	} else {
		// construct based on built-in formula used into controllers.go / executeJob().
		filenameSuffix := fmt.Sprintf("%02d%02d%02d.%s.txt", job.submittime.Year(), job.submittime.Month(), job.submittime.Day(), job.id)
		outputFilePath = filepath.Join(Config.JobsOutputsFolder, filenameSuffix)
	}

	if isDir, err := IsDir(outputFilePath); isDir || err != nil {
		// path could be directory or inexistant.
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, fmt.Sprintf("Sorry, the output file of this job was not found. Check inside the <%s> folder on the server if it is present.", Config.JobsOutputsFolder))
		log.Printf("failed to download job ID [%s] output file. cannot find dump file - errmsg: %v\n", id, err)
		return
	}

	// set type for object streaming.
	w.Header().Set("Content-Type", "application/octet-stream")

	// disable caching.
	w.Header().Set("Cache-Control", "no-cache,no-store,max-age=0,must-revalidate")
	w.Header().Set("Content-Control", "private, no-transform, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache") // HTTP1.0 compatibility.
	w.Header().Set("Expires", "-1")

	// retrieve the filename and send it for downloading. Since the file is already on the disk,
	// http.ServeFile() is useful in this case to automatically load the file size and details.
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(filepath.Base(outputFilePath)))
	http.ServeFile(w, r, outputFilePath)

	// increment number of the job result calls.
	job.lock.Lock()
	job.fetchcount += 1
	job.lock.Unlock()
}
