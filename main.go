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

// Author   : Jerome AMON <Go FullStack Developer>

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func displayVersion() {
	fmt.Printf("\n %s\n\n", infos)
	fmt.Println(" Version ID   :", Version)
	fmt.Println(" Web version  :", WebVersion)
	fmt.Println(" API version  :", APIVersion)
	fmt.Println(" Go version   :", strings.TrimLeft(GoVersion, "go"))
	fmt.Println(" Build Time   :", BuildTime)
	fmt.Println(" Target OS    :", TargetOS+"/"+TargetArch)
	fmt.Println(" Git commit   :", GitCommit)
	fmt.Println(" Author infos :", Author)
	fmt.Println(" Source code  :", SourceLink+GitCommit)
	os.Exit(0)
}

func displayHelp() {
	fmt.Printf("\n%s\n", webv1docs)
	os.Exit(0)
}

func displayUsage() {
	fmt.Printf("\n$$ Usage: %s [command] [--options] [arguments] \n", os.Args[0])
	fmt.Printf("Commands: [start | stop | restart | status | help | version] \n")
	fmt.Printf("Options : [--host <address>] [--port <number>] \n")
	fmt.Println("\nExamples:")
	fmt.Printf("$ %s start --host <address> --port <number> \n", os.Args[0])
	fmt.Printf("$ %s start --host 127.0.0.1 --port 8080 \n", os.Args[0])
	fmt.Printf("$ %s restart --host <address> --port <number> \n", os.Args[0])
	fmt.Printf("$ %s restart --host 127.0.0.1 --port 8080 \n", os.Args[0])
}

func main() {

	if len(os.Args) < 2 {
		// returned the program name back since someone could have
		// compiled with another output name. Then gracefully exit.
		displayUsage()
		os.Exit(0)
	}

	// load and setup worker settings.
	loadConfigFile(DefaultConfigFile)

	// parse and use temporarily provided host & port values.
	startCommand := flag.NewFlagSet("start", flag.ExitOnError)
	startHost := startCommand.String("host", Config.HttpsServerHost, "web server ip address")
	startPort := startCommand.String("port", Config.HttpsServerPort, "web server port number")

	restartCommand := flag.NewFlagSet("restart", flag.ExitOnError)
	restartHost := restartCommand.String("host", Config.HttpsServerHost, "web server ip address")
	restartPort := restartCommand.String("port", Config.HttpsServerPort, "web server port number")

	var err error
	option := strings.ToLower(os.Args[1])

	switch option {
	case "run":
		err = runWorkerService()
	case "start":
		startCommand.Parse(os.Args[2:])
		Config.HttpsServerHost = *startHost
		Config.HttpsServerPort = *startPort
		err = startWorkerService()
	case "restart":
		restartCommand.Parse(os.Args[2:])
		Config.HttpsServerHost = *restartHost
		Config.HttpsServerPort = *restartPort
		err = restartWorkerService()
	case "stop":
		err = stopWorkerService()
	case "status":
		err = checkWorkerService()
	case "help":
		displayHelp()
	case "version":
		displayVersion()
	default:
		// not implemented command.
		fmt.Printf("\nUnknown command: %v\n", os.Args[1])
		displayUsage()
	}

	if err != nil {
		fmt.Printf("%s: error: %v\n", option, err)
		os.Exit(0) // use 0 to avoid output on stdout.
	}

	// below exit will terminate the group leader (current process)
	// so once you close the terminal - the session will be terminated
	// which will force the pseudo deamon to be terminated as well.
	// that is why it is not a real deamon since it does not survives
	// to its parent lifecycle here. So, keep the launcher terminal open.
	os.Exit(0)
}
