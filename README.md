# jobs-worker-service

Cross-platform web & systems backend to execute multiple remote jobs from shell with possibility to specify execution timeout
and fetch the result output and check one or more jobs status and stop one or more running jobs - all done from web interface. 

* Click to watch the live [demo video](https://youtu.be/_xnINhN8MRI)


## Table of contents
* [Description](#description)
* [Features](#features)
* [Technologies](#technologies)
* [Setup](#setup)
* [Usage](#usage)
* [Upcomings](#upcomings)
* [Contribution](#contribution)
* [License](#license)


## Description

This cross-platform web & systems backend allows to execute multiple commands from shell with possibility
to specify single execution timeout for all of the commands submitted. The commands should be submmitted
from web requests. Each command output will be made available into the system memory for further retrieval.
Each command submitted is considered as a unique job with unique identifier. This backend service could be
started - stopped and restarted almost like a unix deamon. Each submitted job could be stopped based on its
unique id. Same approach to check the status. You can even view all submitted jobs status or stop them all.
Finally, you can submit a single command and wait until it completes to view its result output immediately.
Each start of the worker creates a folder to host all three logs files (web requests - jobs - jobs deletion).
There is a single log where the worker will persist its standard output and standard error - worker dot log.


## Features

Below is a summary of current available features. This section will be updated as project continue:

* command-line options to start or stop or restart the worker
* command-line option to check the status of the worker service
* submit one or several jobs from browser or any web client 
* check the status of one or several jobs based on their ids
* request the stop one or several jobs based on their ids
* fetch the execution result of a singlejob based on its id
* request the status of all submitted jobs
* request the stop of all running jobs
* unique output log folder for each worker startup
* single fixed worker log for standard output & error
* per startup web server log and jobs log and deletion cron log 
* https-only local web server with auto-generated self-signed certificate 


Please feel free to have a look at the [usage section](#usage) for examples.



## Technologies

This project is developed with:
* Golang version: 1.13+
* Native libraries only


## Setup

On Windows, Linux macOS, and FreeBSD you will be able to download the pre-built binaries once available.
If your system has [Go >= 1.13+](https://golang.org/dl/) you can pull the codebase and build from the source.

```
# build the worker program on windows
git clone https://github.com/jeamon/wweb-based-jobs-worker-service.git && cd web-based-jobs-worker-service
go build -o worker.exe worker.go help.go

# build the worker program on linux and others
git clone https://github.com/jeamon/web-based-jobs-worker-service.git && cd web-based-jobs-worker-service
go build -o worker worker.go help.go
```


## Usage


```Usage:
    


	                    $$\                                                         $$\                           
	                    $$ |                                                        $$ |                          
	      $$\  $$$$$$\  $$$$$$$\   $$$$$$$\       $$\  $$\  $$\  $$$$$$\   $$$$$$\  $$ |  $$\  $$$$$$\   $$$$$$\  
	      \__|$$  __$$\ $$  __$$\ $$  _____|      $$ | $$ | $$ |$$  __$$\ $$  __$$\ $$ | $$  |$$  __$$\ $$  __$$\ 
	      $$\ $$ /  $$ |$$ |  $$ |\$$$$$$\        $$ | $$ | $$ |$$ /  $$ |$$ |  \__|$$$$$$  / $$$$$$$$ |$$ |  \__|
	      $$ |$$ |  $$ |$$ |  $$ | \____$$\       $$ | $$ | $$ |$$ |  $$ |$$ |      $$  _$$<  $$   ____|$$ |      
	      $$ |\$$$$$$  |$$$$$$$  |$$$$$$$  |      \$$$$$\$$$$  |\$$$$$$  |$$ |      $$ | \$$\ \$$$$$$$\ $$ |      
	      $$ | \______/ \_______/ \_______/        \_____\____/  \______/ \__|      \__|  \__| \_______|\__|      
	$$\   $$ |                                                                                                    
	\$$$$$$  |                                                                                                    
	 \______/


	----------------------------------------------------------------------------------------------
	
			[current version 1.0 By Jerome AMON - cloudmentor.scale@gmail.com]

	----------------------------------------------------------------------------------------------

	[+] Hello - Please find below how to use use this web service.

	----------------------------------------------------------------------------------------------

	[*] Find below command line options dedicated to the worker service :

	[+] On Windows Operating System.
	worker.exe [ start | stop | restart | status | help | version]

	[+] On Linux Operating System.
	./worker [ start | stop | restart | status | help | version]

	----------------------------------------------------------------------------------------------
	
	[1] To execute a remote command and get instantly the output (replace space with + sign):
	
	https://<server-ip-address>:<port>/execute?cmd=<command+argument>
	
	[+] On Windows Operating System.
	example: https://127.0.0.1:8080/execute?cmd=systeminfo
	example: https://127.0.0.1:8080/execute?cmd=ipconfig+/all
	example: https://127.0.0.1:8080/execute?cmd=netstat+-an+|+findstr+ESTAB

	[+] On Linux Operating System.
	example: https://127.0.0.1:8080/execute?cmd=ls+-la
	example: https://127.0.0.1:8080/execute?cmd=ip+a
	example: https://127.0.0.1:8080/execute?cmd=ps

	----------------------------------------------------------------------------------------------

	[2] To submit one or more commands (jobs) for immediate execution and later retreive outputs:
	
	https://<server-ip-address>:<port>/jobs?cmd=<command+argument>&cmd=<command+argument>

	[+] On Windows Operating System.
	example: https://127.0.0.1:8080/jobs?cmd=systeminfo&cmd=ipconfig+/all&cmd=tasklist
	example: https://127.0.0.1:8080/jobs?cmd=ipconfig+/all

	[+] On Linux Operating System.
	example: https://127.0.0.1:8080/jobs?cmd=ls+-la&cmd=ip+a&cmd=ps

	----------------------------------------------------------------------------------------------

	[3] To check the detailed status of one or more submitted commands (jobs):
	
	https://<server-ip-address>:<port>/jobs/status?id=<job-1-id>&id=<job-2-id>

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/jobs/status?id=abe478954cef4125&id=cde478910cef4125

	----------------------------------------------------------------------------------------------

	[4] To fetch the output of one command (job) submitted:
	
	https://<server-ip-address>:<port>/jobs/results?id=<job-id>

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/jobs/results?id=abe478954cef4125

	----------------------------------------------------------------------------------------------

	[5] To check the status of all submitted commands (jobs):
	
	https://<server-ip-address>:<port>/jobs/status/
	https://<server-ip-address>:<port>/jobs/stats/

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/jobs/status/
	example: https://127.0.0.1:8080/jobs/stats/

	----------------------------------------------------------------------------------------------

	[6] To stop of one or more submitted running commands (jobs):
	
	https://<server-ip-address>:<port>/jobs/stop?id=<job-1-id>&id=<job-2-id>

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/jobs/stop?id=abe478954cef4125&id=cde478910cef4125

	----------------------------------------------------------------------------------------------
	
	[7] To stop of all submitted running commands (jobs):
	
	https://<server-ip-address>:<port>/jobs/stop/

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/jobs/stop/

	----------------------------------------------------------------------------------------------
	
```


## Upcomings

* add filter option to display details of completed or stopped or running jobs
* add capability to load configuration details from file at startup
* add capability to restart one or more submitted jobs while keeping their ids
* add option to store jobs result to redis server rather than in-memory map
* add URI and handler to download execution output of one or multiple jobs by ids 
* add command line options on worker service to list or delete jobs or dump output
* add feature to move worker service into maintenance mode - stop accepting jobs


## Contribution

Pull requests are welcome. However, I would be glad to be contacted for discussion before.


## License

please check & read [the license details](https://github.com/jeamon/web-based-jobs-worker/blob/master/LICENSE) or [reach out to me](https://blog.cloudmentor-scale.com/contact) before any action.