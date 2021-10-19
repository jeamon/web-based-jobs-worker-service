package main

// @logging.go contains the function that will create logs folder and all logs file
// for storing web requests, jobs processing events and jobs deletion operations.

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// custom logger for web requests.
var weblog *log.Logger

// custom logger for api calls.
var apilog *log.Logger

// custom logger for tracing jobs deletion.
var deletedjobslog *log.Logger

// custom logger for tracing jobs processing.
var jobslog *log.Logger

// setupLoggers ensures the log folder exist and configures all files needed for logging.
func setupLoggers() {
	// build log folder based on current launch day.
	logsFolderName := fmt.Sprintf("%d%02d%02d.logs", time.Now().Year(), time.Now().Month(), time.Now().Day())
	logsFolderPath := filepath.Join(Config.LogFoldersLocation, logsFolderName)
	// use current day log folder to store all logs files. if the folder does not exist, create it.
	info, err := os.Stat(logsFolderPath)
	if errors.Is(err, os.ErrNotExist) {
		// folder does not exist. create the full nested path.
		err := os.MkdirAll(logsFolderPath, 0755)
		if err != nil {
			// failed to create directory.
			log.Printf("program aborted - failed to create %q logging folder - errmsg: %v", logsFolderPath, err)
			// try to remove PID file.
			os.Remove(Config.WorkerPidFilePath)
			os.Exit(1)
		}
	} else {
		// folder or path already exists but abort if not a directory.
		if !info.IsDir() {
			log.Printf("%q already exists but it is not a folder so check before continue - errmsg: %v\n", logsFolderPath, err)
			// try to remove PID file.
			os.Remove(Config.WorkerPidFilePath)
			os.Exit(0)
		}
	}

	// create the log file for web server requests.
	weblogfile, err := os.OpenFile(filepath.Join(logsFolderPath, Config.WebRequestsLogFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("program aborted - failed to create web requests log file - errmsg: %v\n", err)
		// try to remove PID file.
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(1)
	}
	// setup logging format and parameters.
	weblog = log.New(weblogfile, "", log.LstdFlags|log.Lshortfile|log.LUTC)

	// create the log file for api calls.
	apilogfile, err := os.OpenFile(filepath.Join(logsFolderPath, Config.ApiRequestsLogFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("program aborted - failed to create api calls log file - errmsg: %v\n", err)
		// try to remove PID file.
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(1)
	}
	// setup logging format and parameters.
	apilog = log.New(apilogfile, "", log.LstdFlags|log.Lshortfile|log.LUTC)

	// create file to log deleted jobs by cleanupMapResults goroutine.
	deletedjobslogfile, err := os.OpenFile(filepath.Join(logsFolderPath, Config.JobsDeletionLogFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("program aborted - failed to create jobs deletion log file - errmsg: %v\n", err)
		// try to remove PID file.
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(1)
	}
	// setup logging format and parameters.
	deletedjobslog = log.New(deletedjobslogfile, "", log.LstdFlags|log.Lshortfile|log.LUTC)

	// create file to log jobs related activities.
	jobslogfile, err := os.OpenFile(filepath.Join(logsFolderPath, Config.JobsProcessLogFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("program aborted - failed to create jobs processing log file - errmsg: %v\n", err)
		// try to remove PID file.
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(1)
	}
	// setup logging format and parameters.
	jobslog = log.New(jobslogfile, "", log.LstdFlags|log.Lshortfile|log.LUTC)

	log.Println("logs folder and all log files successfully created.")
}
