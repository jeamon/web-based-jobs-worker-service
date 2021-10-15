package main

// @models.go contains all structure definitions used into this project.

import (
	"bytes"
	"io"
	"sync"
	"time"
)

// Job is a structure for each submitted task. A long running job output could only be streamed
// and/or dumped to a file if specified. It can last for 24hrs (1440 mins). A short running task
// output will only be saved into its memory buffer and can last for 1h (3600s).
type Job struct {
	id          string        // auto-generated 16 hexa job id.
	pid         int           // related job system process id.
	task        string        // full command syntax to be executed.
	islong      bool          // false:(short task) and true:(long task).
	stream      bool          // if true then stream output over websocket.
	dump        bool          // if true then save output to disk file.
	memlimit    int           // maximum allowed memory usage in MB.
	cpulimit    int           // maximum cpu percentage allowed for the job.
	timeout     int           // in secs for short task and mins for long task.
	iscompleted bool          // specifies if job is running or not.
	issuccess   bool          // job terminated without error.
	exitcode    int           // 0success or -1default & stopped.
	errormsg    string        // store any error during execution.
	fetchcount  int           // number of times result queried.
	stop        chan struct{} // helps notify to kill the job process.
	outpipe     io.ReadCloser // job process default std output/error pipe.
	outstream   io.Reader     // makes available output for streaming.
	isstreaming bool          // if stream already being consumed over a websocket.
	lock        *sync.RWMutex // job level mutex for access synchronization.
	result      *bytes.Buffer // in-memory buffer to save output - getJobsOutputById().
	filename    string        // filename where to dump long running job output.
	submittime  time.Time     // datetime job was submitted.
	starttime   time.Time     // datetime job was started.
	endtime     time.Time     // datetime job was terminated.
}

type ApiErrorMessage struct {
	Status    bool   `json:"status"`
	Message   string `json:"message"`
	Code      int    `json:"code"`
	RequestId string `json:"requestid"`
}

// POST /worker/api/v1/jobs/schedule
type RequestJobsSchedule struct {
	Jobs []JobScheduleModel `json:"jobs"`
}

type JobScheduleModel struct {
	Task     string `json:"task"`   // full command syntax to be executed.
	IsLong   bool   `json:"islong"` // short or long running job.
	Stream   bool   `json:"stream"` // output should be live streamed over websocket.
	Dump     bool   `json:"dump"`   // output should be live streamed in unique file.
	MemLimit int    `json:"memlimit,omitempty"`
	CpuLimit int    `json:"cpulimit,omitempty"`
	Timeout  int    `json:"timeout,omitempty"`
}

// Response of POST /worker/api/v1/jobs/schedule
type ResponseJobsSchedule struct {
	RequestId string              `json:"requestid"`
	Jobs      []JobScheduledInfos `json:"jobs"`
}

type JobScheduledInfos struct {
	Id         string `json:"id"`
	Task       string `json:"task"`
	IsLong     bool   `json:"islong"`
	MemLimit   int    `json:"memlimit"`
	CpuLimit   int    `json:"cpulimit"`
	Timeout    int    `json:"timeout"`
	Stream     bool   `json:"stream"`
	Dump       bool   `json:"dump"`
	SubmitTime string `json:"submittime"`
	StatusLink string `json:"statuslink"`
	OutputLink string `json:"outputlink"`
}

// Response of GET /worker/api/v1/jobs/status?id=<jobid>&id=<jobid>&id=<jobid>
type ResponseCheckJobsStatus struct {
	RequestId  string           `json:"requestid"`
	Infos      []JobStatusInfos `json:"infos"`
	UnknownIds []string         `json:"unknownids,omitempty"`
}

type JobStatusInfos struct {
	Id          string `json:"id"`
	Pid         int    `json:"pid"`
	Task        string `json:"task"`
	IsLong      bool   `json:"islong"`
	IsCompleted bool   `json:"iscompleted"`
	IsSuccess   bool   `json:"issuccess"`
	ExitCode    int    `json:"exitcode"`
	DataSize    string `json:"datasize"`
	FetchCount  int    `json:"fetchcount"`
	MemLimit    int    `json:"memlimit"`
	CpuLimit    int    `json:"cpulimit"`
	Timeout     int    `json:"timeout"`
	Stream      bool   `json:"stream"`
	Dump        bool   `json:"dump"`
	SubmitTime  string `json:"submittime"`
	StartTime   string `json:"starttime"`
	EndTime     string `json:"endtime"`
	OutputLink  string `json:"outputlink"`
}

// Response of GET /worker/api/v1/jobs/fetch?id=<jobid>
type ResponseFetchJobOutput struct {
	RequestId string `json:"requestid"`
	Output    string `json:"output"`
}
