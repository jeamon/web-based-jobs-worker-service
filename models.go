package main

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
	dump        bool          // if true then save output to disk file.
	submittime  time.Time     // datetime job was submitted.
	starttime   time.Time     // datetime job was started.
	endtime     time.Time     // datetime job was terminated.
}

// Configuation is the structure of the expected json data to be loaded from the config file <worker.config.json>.
type Configuration struct {
	HttpsServerHost        string `json:"HttpsServerHost"`        // https server ip or dns.
	HttpsServerPort        string `json:"HttpsServerPort"`        // https server port number.
	HttpsServerCerts       string `json:"HttpsServerCerts"`       // https server certificate filename.
	HttpsServerKey         string `json:"HttpsServerKey"`         // https server private key filename.
	HttpsServerCertsPath   string `json:"HttpsServerCertsPath"`   // where to find https server certs/key.
	HttpsServerCertsEmail  string `json:"HttpsServerCertsEmail"`  // email to use when building server certs.
	WorkerPidFilePath      string `json:"WorkerPidFilePath"`      // worker service process identifier file.
	WorkerLogFilePath      string `json:"WorkerLogFilePath"`      // place to store worker process outputs.
	LogFoldersLocation     string `json:"LogFoldersLocation"`     // folder to store daily startup logs.
	WebRequestsLogFile     string `json:"WebRequestsLogFile"`     // file to create and storing all web requests.
	ApiRequestsLogFile     string `json:"ApiRequestsLogFile"`     // file to create for storing all api calls.
	JobsProcessLogFile     string `json:"JobsProcessLogFile"`     // file to create for storing jobs processing lifecycle.
	JobsDeletionLogFile    string `json:"JobsDeletionLogFile"`    // file to create for tracking auto-deleted jobs details.
	DefaultLinuxShell      string `json:"DefaultLinuxShell"`      // shell path to be used on linux platform as last resort.
	JobsOutputsFolder      string `json:"JobsOutputsFolder"`      // folder to create for storing long jobs outputs dump.
	WorkerWorkingDirectory string `json:"WorkerWorkingDirectory"` // path to be used as worker process directory.
	JobsCleanupMaxFetch    int    `json:"JobsCleanupMaxFetch"`    // maximum number of output fetch to allow job deletion.
	JobsCleanupRunInterval int    `json:"JobsCleanupRunInterval"` // each number of hours to check for jobs deletion.
	JobsCleanupMaxAge      int    `json:"JobsCleanupMaxAge"`      // hours since job ended to consider as dead job to delete.
	WaitTimeBeforeExit     int    `json:"WaitTimeBeforeExit"`     // seconds to pause before exiting from the worker.
	ShortJobTimeout        int    `json:"ShortJobTimeout"`        // default and max timeout in seconds for short running jobs.
	LongJobTimeout         int    `json:"LongJobTimeout"`         // default and max timeout in minutes for long running jobs.
	MaxJobsQueueBuffer     int    `json:"MaxJobsQueueBuffer"`     // maximum jobs to queue at once on channel for pickup.
	PidFileWatchInterval   int    `json:"PidFileWatchInterval"`   // each number of mins to check the pidfile presence before self-shutdown.
	EnableWebAccess        bool   `json:"EnableWebAccess"`        // specifies if web routes should be setup or not.
	EnableAPIGateway       bool   `json:"EnableAPIGateway"`       // specifies if api routes should be setup or not.
	MaxNumberOfJobs        int    `json:"MaxNumberOfJobs"`        // maximum number of jobs that could be processed.
}