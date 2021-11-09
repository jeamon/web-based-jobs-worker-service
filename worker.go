package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// buffered (maxJobs) channel to hold submitted (*job).
var globalJobsQueue chan *Job

// store all submitted jobs after processed with job id as key.
var globalJobsResults map[string]*Job
var mapLock *sync.RWMutex

//go:embed assets/websocket.html
var streamPageFS embed.FS
var streamPageTemplate *template.Template

func initializeWorkerSettings() error {
	var err error
	log.Println("initializing default folders and worker settings ...")

	mapLock = &sync.RWMutex{}
	globalJobsQueue = make(chan *Job, Config.MaxJobsQueueBuffer)
	globalJobsResults = make(map[string]*Job)

	// compile the tempplate from the filesystem.
	streamPageTemplate, err = template.ParseFS(streamPageFS, "assets/websocket.html")
	if err != nil {
		log.Printf("failed to initialize streaming web page template - errmsg: %v\n", err)
		return err
	}

	// ensure jobs outputs & backups folder are present.
	err = createFolder(Config.JobsOutputsFolder)
	if err != nil {
		log.Printf("failed to ensure jobs outputs folder - errmsg: %v\n", err)
		return err
	}

	err = createFolder(Config.JobsOutputsBackupsFolder)
	if err != nil {
		log.Printf("failed to ensure jobs outputs backups folder - errmsg: %v\n", err)
		return err
	}

	Config.DefaultJobsInfosTimeLocation, err = time.LoadLocation(Config.DefaultJobsInfosTimezone)
	if err != nil {
		log.Printf("failed to load and convert default timezone value to time location - errmsg: %v\n", err)
		return err
	}

	// for linux-based platform lets find the current shell binary path
	// if environnement shell is set and not empty we use it as default.
	if runtime.GOOS != "windows" {
		if len(os.Getenv("SHELL")) > 0 {
			Config.DefaultLinuxShell = os.Getenv("SHELL")
		}
	}
	// best effort to check & enforce if needed the max number of open files on linux.
	enforceMaxOpenFiles()

	return nil
}

// savePID is a function to create new file and put inside the pid value passed.
func savePID(pid int) error {
	f, err := os.Create(Config.WorkerPidFilePath)
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

	if _, err = os.Stat(Config.WorkerPidFilePath); err != nil {
		return 0, err
	}

	// read the file content.
	data, err := ioutil.ReadFile(Config.WorkerPidFilePath)
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

// runWorkerService is the core function to spin up the worker and all its background services.
func runWorkerService() error {
	runtime.GOMAXPROCS(runtime.NumCPU())
	user, err := user.Current()
	if err != nil {
		log.Printf("failed to retrieve owner name of this worker process - errmsg: %v", err)
		return err
	}

	log.Printf("started worker service. [user: %s] - [pid: %d] - [ppid: %d]\n", user.Username, os.Getpid(), os.Getppid())
	// silently remove pid file if deamon exit.
	defer func() {
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(0)
	}()

	// init global variables and loads configs.
	err = initializeWorkerSettings()
	if err != nil {
		return err
	}

	// to make sure all goroutines exit before leaving the program.
	wg := &sync.WaitGroup{}
	// channel to instruct jobs monitor to exit.
	exit := make(chan struct{}, 1)

	// background handler for interruption signals.
	wg.Add(1)
	go handleSignals(exit, wg)

	wg.Add(1)
	// background jobs map cleaner.
	go cleanupMapResults(Config.JobsCleanupRunInterval, Config.JobsCleanupMaxFetch, Config.JobsCleanupMaxAge, exit, wg)

	wg.Add(1)
	// start the jobs queue monitor.
	go jobsMonitor(exit, wg)

	// setup logs files.
	setupLoggers()

	// run https web server and block until server exits (shutdown).
	err = startWebServer(exit)
	// once server stops - wait until all goroutines exit.
	wg.Wait()
	log.Printf("stopped all goroutines of worker service. [pid: %d] - [user: %s] - [ppid: %d]\n", os.Getpid(), user.Username, os.Getppid())
	return err
}

// checkWorkerService will read stored PID from the local file and will send a
// non impact signal 0 toward that process to check if it exists (running) or not.
func checkWorkerService() (err error) {

	var pid int
	// anonymous function which check final value
	// of pid variable and display deamon status.
	defer func() {
		if pid == 0 {
			fmt.Println("status: not active")
			return
		}
		fmt.Printf("worker service [pid: %d]. status is active\n", pid)
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

	// on windows platform, we use tasklist and findstr to search for the pid.
	// Because syscall.Signal is not available in Windows.
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd", "/C", fmt.Sprintf("tasklist /FI \"PID eq %d\" | findstr %d", pid, pid)).Output()
		if err != nil {
			fmt.Println(err)
			fmt.Printf("process [pid: %d] does not exist. removing pid file: %s\n", pid, Config.WorkerPidFilePath)
			os.Remove(Config.WorkerPidFilePath)
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

	// on linux-based platforms, we send a dummy (non impact) signal 0 to that process.
	// if it responds then it is alive otherwise it is not running.
	if err = process.Signal(syscall.Signal(0)); err != nil && err == syscall.ESRCH {
		fmt.Printf("process [pid: %d] is not running. removing pid file: %s\n", pid, Config.WorkerPidFilePath)
		os.Remove(Config.WorkerPidFilePath)
		// set back to 0 so defer function can display inactive status.
		pid = 0
	}
	return nil
}

// stopWorkerService loads the pid from the file and
// sends a kill signal to the associated process.
func stopWorkerService() error {
	// read the pid from the file.
	pid, err := getPID()
	if err != nil {
		if os.IsNotExist(err) {
			// the file does not exist.
			log.Printf("cannot find pid file [%q] to load the worker service pid.\n", Config.WorkerPidFilePath)
			return nil
		}
		log.Printf("cannot retrieve the pid of the worker service. check if [%q] is present and there is worker process with the pid.\n", Config.WorkerPidFilePath)
		return err
	}

	// try to locate the process.
	process, err := os.FindProcess(pid)
	if err != nil {
		log.Printf("cannot find a running worker process [pid: %v]\n", pid)
		if runtime.GOOS == "windows" {
			// on windows only platforms.
			log.Printf("1/ Open the windows console with administrator privileges")
			log.Printf("2/ check if the worker is running with <tasklist /FI \"PID eq %v\">", pid)
			log.Printf("3/ if it is running, try to force stop it with <taskkill /F /PID %v /T>", pid)
		} else {
			// any other unix-like OSes.
			log.Printf("1/ Open the console with privileges to view & stop process")
			log.Printf("2/ check if the worker is running with <ps -p %v>", pid)
			log.Printf("3/ if it is running, try to stop it with <kill -9 %v>", pid)
		}
		return err
	}
	// remove the PID file
	os.Remove(Config.WorkerPidFilePath)
	if err := process.Kill(); err != nil {
		log.Printf("failed to kill worker service [pid: %v]\n", pid)
		// some tips to check and stop the worker process manually.
		if runtime.GOOS == "windows" {
			// on windows only platforms.
			log.Printf("1/ Open the windows console with administrator privileges")
			log.Printf("2/ check if the worker is running with <tasklist /FI \"PID eq %v\">", pid)
			log.Printf("3/ if it is running, try to force stop it with <taskkill /F /PID %v /T>", pid)
		} else {
			// any other unix-like OSes.
			log.Printf("1/ Open the console with privileges to view & stop process")
			log.Printf("2/ check if the worker is running with <ps -p %v>", pid)
			log.Printf("3/ if it is running, try to stop it with <kill -9 %v>", pid)
		}
		return err
	} else {
		log.Printf("successfully stopped worker service [pid: %v]\n", pid)
		return nil
	}
}

// restartWorkerService restarts the worker service process.
func restartWorkerService() error {
	// read the pid from the file.
	pid, err := getPID()
	if err != nil {
		if os.IsNotExist(err) {
			// the file does not exist. start deamon
			return startWorkerService()
		}
		// file may exist. remove it and start
		os.Remove(Config.WorkerPidFilePath)
		return startWorkerService()
	}

	// existig pid retreived so remove the file and kill process
	os.Remove(Config.WorkerPidFilePath)
	process, err := os.FindProcess(pid)
	if err != nil {
		// did not find process - may has exited. start
		return startWorkerService()
	}

	if err := process.Kill(); err != nil {
		// failed to kill existing process
		fmt.Printf("failed to kill worker service [pid: %v]\n", pid)
		return err
	}
	// succeed to kill existing - so start new.
	return startWorkerService()
}

// startWorkerService calls and executes runDeamon() as a parallel process.
func startWorkerService() error {
	// check if the deamon is already running.
	if _, err := os.Stat(Config.WorkerPidFilePath); err == nil {
		fmt.Printf("worker service is already running or the pid file [%s] already exists. try to restart.\n", Config.WorkerPidFilePath)
		return err
	}

	// launch the program with deamon as argument
	// syscall exec is not the same as fork.
	cmd := exec.Command(os.Args[0], "run")
	// setup the process working directory.
	cmd.Dir = Config.WorkerWorkingDirectory
	// single file to log output of worker - read by all and write only by the user.
	workerlog, err := os.OpenFile(Config.WorkerLogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("failed to create or open worker process log file", Config.WorkerLogFilePath)
		return err
	}

	// send worker process output & error to its log file.
	cmd.Stderr, cmd.Stdout = workerlog, workerlog

	// start the process asynchronously.
	if err := cmd.Start(); err != nil {
		fmt.Println("failed to start the worker service process")
		return err
	}

	// save the worker service pid to disk file.
	if err := savePID(cmd.Process.Pid); err != nil {
		// try to kill the process
		if err := cmd.Process.Kill(); err != nil {
			fmt.Printf("failed to kill the worker service process [pid: %d]\n", cmd.Process.Pid)
		}

		return err
	}
	// speudo deamon started
	log.Printf("started worker service [pid: %d] - [ppid: %d]\n", cmd.Process.Pid, os.Getpid())

	// below exit will terminate the group leader (current process)
	// so once you close the terminal - the session will be terminated
	// which will force the pseudo deamon to be terminated as well.
	// that is why it is not a real deamon since it does not survives
	// to its parent lifecycle here.
	// os.Exit(0)
	return nil
}
