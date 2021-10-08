package main

// @api-handlers.go contains functions that directly handle each restful api calls.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// apiScheduleJobs schedules for processing a list of jobs submitted. POST /worker/api/v1/jobs/schedule/
func apiScheduleJobs(w http.ResponseWriter, r *http.Request) {

	requestid := getFromRequest(r, "requestid")
	if r.Method != "POST" {
		SendErrorMessage(w, requestid, "Failed to schedule job(s). Invalid request method.", http.StatusMethodNotAllowed)
		return
	}

	var err error
	var data RequestJobsSchedule
	// Decode the json list of jobs.
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		SendErrorMessage(w, requestid, "Failed to schedule job(s). Invalid request content format. Make sure to fill all required fields.", http.StatusBadRequest)
		return
	}

	jobs := data.Jobs

	if len(jobs) == 0 {
		// nothing submitted into request body.
		SendErrorMessage(w, requestid, "Failed to schedule job(s). No jobs found in the resquest.", http.StatusBadRequest)
		return
	}

	var finalJobsInfos []JobScheduledInfos
	var memlimit int
	var cpulimit int
	var timeout int

	for _, jobInfos := range jobs {

		timeout = jobInfos.Timeout

		if jobInfos.IsLong {
			if timeout <= 0 || timeout >= Config.LongJobTimeout {
				// default to long job timeout.
				timeout = Config.LongJobTimeout
			}

		} else {
			// short job.
			if timeout <= 0 || timeout >= Config.LongJobTimeout {
				// default to long job timeout.
				timeout = Config.ShortJobTimeout
			}

			if jobInfos.Dump {
				// fix the mistake. dump is for long job.
				jobInfos.Dump = false
			}
		}

		if jobInfos.CpuLimit > 0 && jobInfos.CpuLimit <= Config.CpuLimitMaxPercentage {
			cpulimit = jobInfos.CpuLimit
		} else {
			cpulimit = Config.CpuLimitDefaultPercentage
		}

		if jobInfos.MemLimit > 0 && jobInfos.MemLimit <= Config.MemoryLimitMaxMegaBytes {
			memlimit = jobInfos.MemLimit
		} else {
			memlimit = Config.MemoryLimitDefaultMegaBytes
		}

		job := &Job{
			id:          generateID(),
			pid:         0,
			task:        jobInfos.Task,
			islong:      jobInfos.IsLong,
			iscompleted: false,
			issuccess:   false,
			exitcode:    -1,
			errormsg:    "",
			fetchcount:  0,
			stop:        make(chan struct{}, 1),
			lock:        &sync.RWMutex{},
			dump:        jobInfos.Dump,
			memlimit:    memlimit,
			cpulimit:    cpulimit,
			timeout:     timeout,
			submittime:  time.Now().UTC(),
			starttime:   time.Time{},
			endtime:     time.Time{},
		}

		if !jobInfos.IsLong {
			// short job, so initialize result buffer.
			job.result = new(bytes.Buffer)
		}

		// add scheduled job details to response list.
		finalJobsInfos = append(finalJobsInfos, job.scheduledInfos())
		// register job to global map results so user can check status while job being executed.
		mapLock.Lock()
		globalJobsResults[job.id] = job
		mapLock.Unlock()
		// add this job to the processing queue.
		globalJobsQueue <- job
		jobslog.Printf("[%s] [%05d] scheduled the processing of the job\n", job.id, job.pid)
	}

	// send response data into json format.
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(ResponseJobsSchedule{RequestId: requestid, Jobs: finalJobsInfos})
	if err != nil {
		apilog.Printf("[request:%s] failed to send jobs scheduled infos - errmsg: %v\n", requestid, err)
		return
	}

	apilog.Printf("[request:%s] success to send jobs scheduled infos\n", requestid)
}

// apiCheckJobsStatusById sends one or multiple jobs full current details.
// GET /worker/api/v1/jobs/status?id=<jobid>&id=<jobid>&id=<jobid>
func apiCheckJobsStatusById(w http.ResponseWriter, r *http.Request) {

	requestid := getFromRequest(r, "requestid")
	if r.Method != "GET" {
		SendErrorMessage(w, requestid, "Failed to check job(s) status. Invalid request method.", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	ids, exist := query["id"]
	if !exist || len(ids) == 0 {
		// request does not contains query string id.
		SendErrorMessage(w, requestid, "Failed to check job(s) status. Invalid request format or no jobs ids submitted.", http.StatusBadRequest)
		return
	}

	// cleanup user submitted list of jobs ids.
	if ignore := removeDuplicateJobIds(&ids); ignore {
		// no remaining good job id.
		SendErrorMessage(w, requestid, "Failed to check job(s) status. The request sent does not contain any valid job id after verification.", http.StatusBadRequest)
		return
	}

	// retreive each job status and send.
	unknownIds := make([]string, 0)
	var jobsStatusInfos []JobStatusInfos

	mapLock.RLock()
	for _, id := range ids {
		job, exist := globalJobsResults[id]

		if !exist {
			// job id does not exist.
			unknownIds = append(unknownIds, id)
			continue
		}
		// add job status details to response list.
		jobsStatusInfos = append(jobsStatusInfos, job.collectStatusInfos())
	}
	mapLock.RUnlock()

	// send response data into json format.
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(ResponseCheckJobsStatus{RequestId: requestid, Infos: jobsStatusInfos, UnknownIds: unknownIds})
	if err != nil {
		apilog.Printf("[request:%s] failed to send jobs status - errmsg: %v\n", requestid, err)
		return
	}

	apilog.Printf("[request:%s] success to send jobs status\n", requestid)
}

// apiCheckAllJobsStatus sends all jobs full current details.
// GET /worker/api/v1/jobs/status/
func apiCheckAllJobsStatus(w http.ResponseWriter, r *http.Request) {

	requestid := getFromRequest(r, "requestid")
	if r.Method != "GET" {
		SendErrorMessage(w, requestid, "Failed to check job(s) status. Invalid request method.", http.StatusMethodNotAllowed)
		return
	}

	// retreive each job status and send.
	var jobsStatusInfos []JobStatusInfos

	mapLock.RLock()
	for _, job := range globalJobsResults {
		// add job status details to response list.
		jobsStatusInfos = append(jobsStatusInfos, job.collectStatusInfos())
	}
	mapLock.RUnlock()

	// send response data into json format.
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(ResponseCheckJobsStatus{RequestId: requestid, Infos: jobsStatusInfos})
	if err != nil {
		apilog.Printf("[request:%s] failed to send all jobs status - errmsg: %v\n", requestid, err)
		return
	}

	apilog.Printf("[request:%s] success to send all jobs status\n", requestid)
}

// apiFetchJobsOutputById retreives result output for a given short job.
func apiFetchJobsOutputById(w http.ResponseWriter, r *http.Request) {

	requestid := getFromRequest(r, "requestid")
	if r.Method != "GET" {
		SendErrorMessage(w, requestid, "Failed to fetch job output. Invalid request method.", http.StatusMethodNotAllowed)
		return
	}

	// expect one value for the query - if multiple passed only first will be returned.
	id := r.URL.Query().Get("id")
	// make sure id provided matche - 16 hexa characters.
	if match, _ := regexp.MatchString(`[a-z0-9]{16}`, id); !match {
		SendErrorMessage(w, requestid, "Failed to fetch job output. Invalid job id.", http.StatusBadRequest)
		return
	}

	// get read lock and verify existence of job into the results map.
	mapLock.RLock()
	job, exist := globalJobsResults[id]
	mapLock.RUnlock()
	if !exist {
		SendErrorMessage(w, requestid, "Failed to fetch job output. Job id provided does not exits. Could have been removed or expired", 404)
		return
	}

	if job.islong {
		SendErrorMessage(w, requestid, "Failed to fetch job output. This job is a long running job. You must stream the output over websocket.", http.StatusBadRequest)
		return
	}

	// increment number of the job result calls.
	job.lock.Lock()
	job.fetchcount += 1
	job.lock.Unlock()

	// send response data into json format.
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(ResponseFetchJobOutput{RequestId: requestid, Output: (job.result).String()})
	if err != nil {
		apilog.Printf("[request:%s] failed to send job output - errmsg: %v\n", requestid, err)
		return
	}

	apilog.Printf("[request:%s] success to send job output\n", requestid)
}
