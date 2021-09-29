package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// characteristics to apply when moving to connection to websocket.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var connectionUpgradeRegex = regexp.MustCompile("(^|.*,\\s*)upgrade($|\\s*,)")
var tmpl *template.Template

func init() {
	// compile stream web page template.
	tmpl = template.Must(template.ParseFiles("websocket.html"))
	// create job outputs files.
	createOutputsFolder()
}

// createOutputsFolder makes sure that "outputs" folder if present - if not create it.
func createOutputsFolder() {
	info, err := os.Stat("outputs")
	if errors.Is(err, os.ErrNotExist) {
		// path does not exist.
		err := os.Mkdir("outputs", 0755)
		if err != nil {
			log.Printf("failed create %q folder - errmsg : %v\n", "outputs", err)
			// try to remove PID file.
			os.Remove(pidFile)
			os.Exit(1)
		}
	} else {
		// path already exists but could be file or directory.
		if !info.IsDir() {
			log.Printf("path %q exists but it is not a folder so please check before continue - errmsg : %v\n", "outputs", err)
			// try to remove PID file.
			os.Remove(pidFile)
			os.Exit(0)
		}
	}
}

// serveStreamPage delivers the web page which contains javascript code to initiate websocket connection.
func serveStreamPage(w http.ResponseWriter, r *http.Request) {
	// expect one value for the query.
	id := r.URL.Query().Get("id")
	// make sure id provided matche - 16 hexa characters.
	if match, _ := regexp.MatchString(`[a-z0-9]{16}`, id); !match {
		w.WriteHeader(400)
		fmt.Fprintln(w, "Sorry, the Job ID provided is invalid.")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf8")
	info := map[string]interface{}{
		"id":     id,
		"server": r.Host,
	}
	err := tmpl.Execute(w, info)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func isWebsocketRequest(req *http.Request) bool {
	return connectionUpgradeRegex.MatchString(strings.ToLower(req.Header.Get("Connection"))) && strings.ToLower(req.Header.Get("Upgrade")) == "websocket"
}

// streamJobsOutputById streams the result output for a given running job.
func streamJobsOutputById(w http.ResponseWriter, r *http.Request) {
	if isWebsocketRequest(r) {
		// websocket connection to stream the output.
		// upgrade to a WebSocket connection.
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			jobslog.Println(err)
			return
		}
		defer ws.Close()
		// retrieve the wanted job id and make sure it is valid.
		id := r.URL.Query().Get("id")
		if match, _ := regexp.MatchString(`[a-z0-9]{16}`, id); !match {
			ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\nSorry, this Job ID [%s] provided into the request is not valid.", id)))
			return
		}
		// Handle websockets if specified.
		streamJob(ws, id)
		return

	} else {
		// non-websockets connection.
		serveStreamPage(w, r)
		return
	}
}

func streamJob(ws *websocket.Conn, id string) {
	// verify existence of the job.
	mapLock.RLock()
	job, exist := globalJobsResults[id]
	mapLock.RUnlock()

	if !exist {
		// job does not exit, no need to continue.
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\nSorry, this Job ID [%s] provided does not exist - could have been removed or expired", id)))
		return
	}

	if job.iscompleted {
		// job already finished, pull the result rather than stream.
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\nThe job is no longer running - pull its full output from /jobs/results?id=%s", id)))
		return
	}

	if !job.islong {
		// job is not a long running task for stream, pull the result rather than stream.
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\nThe job was not submitted as a long running task - pull its full output from /jobs/results?id=%s", id)))
		return
	}

	job.lock.RLock()
	if job.isstreaming {
		defer job.lock.RUnlock()
		// job output streaming is already being consumed.
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\nThe job output stream is already being consumed. Only one live streaming at a moment or you can stop the job.")))
		return
	}
	job.lock.RUnlock()

	// long job still running and no one is consuming the stream so start streaming.
	jobslog.Printf("[%s] [%05d] starting the output streaming of the job\n", id, job.pid)
	// flag stream in use and ensure to unset once this handler exit.
	job.lock.Lock()
	job.isstreaming = true
	job.fetchcount += 1
	job.lock.Unlock()
	defer func() {
		job.lock.Lock()
		job.isstreaming = false
		job.lock.Unlock()
	}()

	var reader *bufio.Reader
	if job.dump {
		// construct a reader from job level TeeReader stream.
		reader = bufio.NewReader(job.outstream)
	} else {
		// construct a reader from process output pipe.
		reader = bufio.NewReader(job.outpipe)
	}
	for {
		// read line by line the pipe content (include the newline char) and stream it.
		b, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF && job.iscompleted {
				// expected since job finished.
				jobslog.Printf("[%s] [%05d] completed the streaming and processing of the job\n", id, job.pid)
				ws.WriteMessage(websocket.TextMessage, []byte("\n\nJob completed. No more stream."))
				return
			} else if err != io.EOF && job.iscompleted == false {
				// error happened during stream reading. close this streaming session.
				jobslog.Printf("[%s] [%05d] error when reading output stream of the job - errmsg: %v\n", id, job.pid, err)
				ws.WriteMessage(websocket.TextMessage, []byte("\n\nOoops, error occured. Job might still be running. Retry the streaming or stop/restart the job."))
				return
			} else {
				// unexpected error - must be reported for investigation.
				// error happened during stream reading. close this streaming session.
				jobslog.Printf("[%s] [%05d] unexpected error when reading output stream of the job - errmsg: %v\n", id, job.pid, err)
				ws.WriteMessage(websocket.TextMessage, []byte("\n\nOoops, unexpected error occured on server side. Retry the streaming or report for investigation."))
				return
			}
		}

		err = ws.WriteMessage(websocket.TextMessage, b)
		if err != nil {
			jobslog.Printf("[%s] [%05d] error when sending output stream of the job - errmsg: %v\n", id, job.pid, err)
			return
		}
	}
}

// scheduleLongJobsWithStreaming receives and schedules only long running jobs submitted by the user.
// it automatically set the job data structure <islong> to true. if multiple jobs are submitted
// into a single request then it ignores the query string <dump> so that the output will not be
// saved on disk. Then will return a summary of jobs submitted. For a single submitted job, it
// will set the <dump> to true or false if mentionned into the request query string. The default
// value for <dump> field is false. Then it will redirect your browser to the streaming page.
func scheduleLongJobsWithStreaming(w http.ResponseWriter, r *http.Request) {
	// parse all query strings.
	query := r.URL.Query()
	cmds, exist := query["cmd"]
	if !exist || len(cmds) == 0 {
		w.Header().Set("Content-Type", "text/plain; charset=utf8")
		// request does not contains query string cmd.
		w.WriteHeader(400)
		fmt.Fprintf(w, "\n[-] Sorry, the request submitted is malformed, go to http://<server-ip>:8080/ to view detailed help.")
		return
	}
	// default memory (megabytes) and cpu (percentage) limit values.
	memlimit := 100
	cpulimit := 10
	timeout := longJobTimeout
	dump := false
	// extract only first value of mem and cpu query string.
	if m, err := strconv.Atoi(query.Get("mem")); err == nil && m > 0 {
		memlimit = m
	}

	if c, err := strconv.Atoi(query.Get("cpu")); err == nil && c > 0 {
		cpulimit = c
	}
	// retreive timeout parameter value and consider it if higher than 0.
	if t, err := strconv.Atoi(query.Get("timeout")); err == nil && t > 0 && t <= longJobTimeout {
		timeout = t
	}

	if len(cmds) == 1 {
		// single job, so constructs job and schedule it then redirect (301) user to streaming page.
		if d := query.Get("dump"); strings.ToLower(d) == "true" {
			dump = true
		}
		job := &Job{
			id:          generateID(),
			pid:         0,
			task:        cmds[0],
			islong:      true,
			iscompleted: false,
			issuccess:   false,
			exitcode:    -1,
			errormsg:    "",
			fetchcount:  0,
			stop:        make(chan struct{}, 1),
			isstreaming: false,
			lock:        &sync.RWMutex{},
			result:      new(bytes.Buffer),
			dump:        dump,
			memlimit:    memlimit,
			cpulimit:    cpulimit,
			timeout:     timeout,
			submittime:  time.Now().UTC(),
			starttime:   time.Time{},
			endtime:     time.Time{},
		}
		// register job to global map results so user can control it.
		mapLock.Lock()
		globalJobsResults[job.id] = job
		mapLock.Unlock()
		// add this job to the processing queue.
		globalJobsQueue <- job
		jobslog.Printf("[%s] [%05d] scheduled the processing of the job\n", job.id, job.pid)
		http.Redirect(w, r, fmt.Sprintf("https://%s/worker/v1/jobs/long/output/stream?id=%s", r.Host, job.id), http.StatusMovedPermanently)
		return
	}

	// multiple jobs so schedule all of them and send summary.
	// try to setup the response as not buffered data.
	f, ok := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/plain; charset=utf8")
	w.WriteHeader(200)
	w.Write([]byte("\n[+] find below some details of the jobs submitted\n\n"))

	// format the display table.
	title := fmt.Sprintf("|%-4s | %-18s | %-14s | %-10s | %-7s | %-20s | %-30s |", "Nb", "Job ID", "Memory [MB]", "CPU [%]", "Timeout", "Submitted At [UTC]", "Command Syntax")
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(14), Dashs(10), Dashs(7), Dashs(20), Dashs(30)))
	if ok {
		f.Flush()
	}
	// build each job per command with resources limit values.
	for i, cmd := range cmds {
		job := &Job{
			id:          generateID(),
			pid:         0,
			task:        cmd,
			islong:      true,
			iscompleted: false,
			issuccess:   false,
			exitcode:    -1,
			errormsg:    "",
			fetchcount:  0,
			stop:        make(chan struct{}, 1),
			isstreaming: false,
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
		// register job to global map results so user can control it.
		mapLock.Lock()
		globalJobsResults[job.id] = job
		mapLock.Unlock()
		// add this job to the processing queue.
		globalJobsQueue <- job
		jobslog.Printf("[%s] [%05d] scheduled the processing of the job\n", job.id, job.pid)
		// stream the added job details to user/client.
		fmt.Fprintln(w, fmt.Sprintf("|%04d | %-18s | %-14d | %-10d | %-7d | %-20v | %-30s |", i+1, job.id, job.memlimit, job.cpulimit, job.timeout, (job.submittime).Format("2006-01-02 15:04:05"), truncateSyntax(job.task, 30)))
		fmt.Fprintf(w, fmt.Sprintf("+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(18), Dashs(14), Dashs(10), Dashs(7), Dashs(20), Dashs(30)))
		if ok {
			f.Flush()
		}
	}
}
