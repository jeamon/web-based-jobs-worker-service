# jobs-worker-service

Cross-platform web & systems backend to execute multiple remote jobs from shell with possibility to specify execution timeout
and fetch the result output and check one or more jobs status and stop one or more running jobs - all done from web interface. 



## Table of contents
* [Description](#description)
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
unique id. Same approach to check the status. You can even view all submitted jobs status. Finally, you can
submit a single command and wait until its execution ends in order to view its result output immediately.
Each start of the worker creates a folder to host all three logs files (web requests - jobs - jobs deletion).
Please feel free to have a look at the [usage section](#usage) for examples.



## Technologies

This project is developed with:
* Golang version: 1.13+
* Native libraries only


## Setup

On Windows, Linux macOS, and FreeBSD you will be able to download the pre-built binaries once available.
If your system has [Go >= 1.7](https://golang.org/dl/) you can pull the codebase and build from the source.

```
# build the cli-streamer program on windows
git clone https://github.com/jeamon/web-based-jobs-worker.git && cd web-based-jobs-worker
go build -o worker.exe worker.go help.go

# build the cli-streamer program on linux and others
git clone https://github.com/jeamon/web-based-jobs-worker.git && cd web-based-jobs-worker
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
	
	http://<server-ip-address>:<port>/execute?cmd=<command+argument>
	
	[+] On Windows Operating System.
	example: http://127.0.0.1:8080/execute?cmd=systeminfo
	example: http://127.0.0.1:8080/execute?cmd=ipconfig+/all
	example: http://127.0.0.1:8080/execute?cmd=netstat+-an+|+findstr+ESTAB

	[+] On Linux Operating System.
	example: http://127.0.0.1:8080/execute?cmd=ls+-la
	example: http://127.0.0.1:8080/execute?cmd=ip+a
	example: http://127.0.0.1:8080/execute?cmd=ps

	----------------------------------------------------------------------------------------------

	[2] To submit one or more commands (jobs) for immediate execution and later retreive outputs:
	
	http://<server-ip-address>:<port>/jobs?cmd=<command+argument>&cmd=<command+argument>

	[+] On Windows Operating System.
	example: http://127.0.0.1:8080/jobs?cmd=systeminfo&cmd=ipconfig+/all&cmd=tasklist
	example: http://127.0.0.1:8080/jobs?cmd=ipconfig+/all

	[+] On Linux Operating System.
	example: http://127.0.0.1:8080/jobs?cmd=ls+-la&cmd=ip+a&cmd=ps

	----------------------------------------------------------------------------------------------

	[3] To check the detailed status of one or more submitted commands (jobs):
	
	http://<server-ip-address>:<port>/jobs/status?id=<job-1-id>&id=<job-2-id>

	[+] On Windows or Linux Operating System.
	example: http://127.0.0.1:8080/jobs/status?id=abe478954cef4125&id=cde478910cef4125

	----------------------------------------------------------------------------------------------

	[4] To fetch the output of one command (job) submitted:
	
	http://<server-ip-address>:<port>/jobs/results?id=<job-id>

	[+] On Windows or Linux Operating System.
	example: http://127.0.0.1:8080/jobs/results?id=abe478954cef4125

	----------------------------------------------------------------------------------------------

	[5] To check the status of all submitted commands (jobs):
	
	http://<server-ip-address>:<port>/jobs/status/
	http://<server-ip-address>:<port>/jobs/stats/

	[+] On Windows or Linux Operating System.
	example: http://127.0.0.1:8080/jobs/status/
	example: http://127.0.0.1:8080/jobs/stats/

	----------------------------------------------------------------------------------------------
	
```


## Upcomings

* add capability to stop one or more running jobs
* add capability to load configuration details from file at startup
* add capability to restart one or more submitted jobs
* extend context cancellation capability to all goroutines
* add new submitted job to result store even not completed
* add HTTPS support and option to start HTTP or HTTPS server 


## Contribution

Pull requests are welcome. However, I would be glad to be contacted for discussion before.


## License

please check & read [the license details](https://github.com/jeamon/web-based-jobs-worker/blob/master/LICENSE) or [reach out to me](https://blog.cloudmentor-scale.com/contact) before any action.