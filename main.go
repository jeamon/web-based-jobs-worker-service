package main

// This cross-platform web & api-driven backend allows to execute multiple commands/jobs from shell with the
// option to specify execution timeout, memory and cpu usage limits. The jobs could be submmitted either
// from your web browser or by an API call. Each job output could be stored into the system memory or on disk
// for further fetching/downloading. It is possible to submit a long running job and live streaming its output.
// Each task submitted is considered as a unique job with unique identifier. This backend service could be
// started - stopped and restarted almost like a unix deamon. Each submitted job could be stopped based on its
// unique id. Same approach to check the status. You can even view all submitted jobs status. Finally, you can
// submit a single job and dual streaming its execution output immediately over a websocket and to a disk file.
// Each start of the worker creates a folder to host all three logs files (web requests - jobs - jobs deletion).

// https://blog.cloudmentor-scale.com/contact

// Version  : 1.1
// Author   : Jerome AMON
// Created  : 20 August 2021

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

func displayVersion() {
	fmt.Printf("\n%s\n", version)
	os.Exit(0)
}

func displayHelp() {
	fmt.Printf("\n%s\n", webv1docs)
	os.Exit(0)
}

// handleSignal is a function that process SIGTERM from kill command or CTRL-C or more.
func handleSignal(exit chan struct{}, wg *sync.WaitGroup) {
	log.Println("started goroutine for exit signal handling ...")
	defer wg.Done()
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL,
		syscall.SIGTERM, syscall.SIGHUP, os.Interrupt, os.Kill, syscall.SIGABRT)

	// block until something comes in.
	signalType := <-sigch
	signal.Stop(sigch)
	log.Println("received exit command [signal: %v]. stopping worker service.", signalType)
	// perform cleanup action : remove pid file.
	os.Remove(Config.WorkerPidFilePath)
	// close quit channel, so jobs monitor goroutine will be notified.
	close(exit)
	return
}

func main() {

	if len(os.Args) != 2 {
		// returned the program name back since someone could have
		// compiled with another output name. Then gracefully exit.
		fmt.Printf("Usage: %s [start|stop|restart|status|help|version] \n", os.Args[0])
		os.Exit(0)
	}

	var err error
	option := strings.ToLower(os.Args[1])

	// load and setup worker settings.
	loadConfigFile(DefaultConfigFile)

	switch option {
	case "run":
		err = runWorkerService()
	case "start":
		err = startWorkerService()
	case "stop":
		err = stopWorkerService()
	case "status":
		err = checkWorkerService()
	case "restart":
		err = restartWorkerService()
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
		log.Println(option, "error:", err)
		os.Exit(1)
	}

	// below exit will terminate the group leader (current process)
	// so once you close the terminal - the session will be terminated
	// which will force the pseudo deamon to be terminated as well.
	// that is why it is not a real deamon since it does not survives
	// to its parent lifecycle here. So, keep the launcher terminal open.
	os.Exit(0)
}
