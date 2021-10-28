# web-based-jobs-worker-service

A go-based fast backend service to spin up and control (stop / fetch / live-stream / download / restart / delete) multiple remote jobs (applications or shell commands)
with the capabilities to specify the execution timeout and resources limitations for these jobs. You perform anything from your usual web browser or by RestFul APIs. 

* General Features Overview - Click to watch the live [demo video](https://youtu.be/_xnINhN8MRI)
* Dual Streaming Feature - Click to watch the live [demo video](https://youtu.be/NsM7AhWkCko)


## Table of contents
* [Description](#description)
* [Features](#features)
* [Technologies](#technologies)
* [Setup](#setup)
* [Get Started](#get-started)
* [Web Access](#web-access)
* [Rest APIs](#rest-apis)
* [Upcomings](#upcomings)
* [Contribution](#contribution)
* [License](#license)


## Description

You must launch the worker service by specifying an option. The available options are start - stop - restart. Once the worker started, it saves its process id (pid)
into a local file named deamon(.)pid since this pid value will be used later to kill the process when you request to stop the worker service. The worker service will
spin up some background procedures (goroutines) such as a monitoring service (cleanupMapResults - to cleanup after a certain time some completed jobs), a signal handler
service (handleSignal - to process any system call exit signals) and a monitoring jobs queue service (jobsMonitor - to watch the jobs queue/channel and start processing).

After creating the dedicated logs folder and files it loads its self-signed certificate and rsa-based private key. In case these files are not present into the certs
folder, they will be created automatically with configurable parameters. This certificate is used by the web service to setup its TLS configurations.

Once the web server is up & running, you can submit one or multiple jobs for remote execution. For each job, a unique id (16 hexadecimal characters) will be generated to
identify this job, so it can be used to check the status or stop or restart or fetch or live-streaming the output of that job. When submitted a job you must specify if the
output should be buffered into memory or streamed over a websocket or/and towards a file. By default a short running job output is saved into memory buffer. Any job output
saved into a file during its processing could be downloaded as well.

All jobs could be stopped or restarted as well. When a job restart is requested, in case this job is still running, it will only be stopped and you will need to restart it
again in order to have it scheduled for processing. If already completed it will be immediately scheduled for processing.

Finally, you can submit a single job and wait until it completes to view its result output immediately. Or you can submit a long running job and view in realtime (in streaming)
it log output. Each start of the worker service creates three logs files. One file logs all web requests, another for each job processing details and the third one for recording
each job details deleted by the cleanup background routine. The worker itself logs its standard output and standard error into a single file named worker(.)log. You can view all
the features APIs exposed by the web server under the [usage section](#usage).


## Features

Below is a summary of available features. This section will be updated as project continue:

* auto-generated and daily grouping of log folders done at each worker startup
* single fixed worker log for standard output & error
* per startup web server log and jobs log and deletion cron log
* command-line options to start or stop or restart the worker
* command-line option to check the status of the worker service
* submit one or several jobs from browser or any web client 
* check the status of one or several jobs based on their ids
* request the stop of one or several jobs based on their ids
* fetch the execution result of a singlejob based on its id
* request the status or the stop of all submitted jobs
* https-only local web server with auto-generated self-signed certificate 
* capability to restart one or more jobs by their ids or restart all jobs
* schedule a long running task and start viewing the job output in realtime
* schedule multiple long running jobs and use their id for live streaming their output
* schedule a long running task with dual streaming (to a disk file and over websocket)
* schedule multiple long running jobs and stream their output to disk files only
* restful apis to schedule multiple jobs and check their status and fetch their output
* dump max-aged jobs output to disk file before deleting them from the result map
* tag each web or api request/call with a unique request id for tracing & debugging
* auto-generated json-based configuration file containing the worker settings
* user-defined foreground & background colors & font-size & font-weight of streaming web page
* capability to specify in config file if Web and/or API URLS should be enabled or disabled
* capability to download the output dump file of a long running job (scheduled with dump option)
* user-defined settings option to specify if logs entries and requests id timestamp must use UTC or local TZ
* embedded websocket/streaming html template file into the executable by leveraging golang <embed> feature
* capability to check worker liveness (ping/pong) and API/WEB routes availability and pull its settings


Please feel free to have a look at the [Web Access](#web-access) and [Rest APIs](#rest-apis) for practical examples.



## Technologies

This project is developed with below packages. It has been designed to be & stay simpler as much as possible.
This philosophy leads to have a single main package with multiple self-described files for code organization. 
Thus makes it easier to move as you wish to a more garnular packaging approach during codebase refactoring.

* Golang version: 1.16+
* Heavy usage of Golang Native packages

**net/http - context - html/template - io - log - os/exec - encoding/json - errors**
**runtime - sync - syscall - regexp - bufio - os/signal - sort - bytes - unicode/utf8**
**crypto/tls - crypto/rand - crypto/rsa - crypto/x509 - crypto/x509/pkix - encoding/pem etc.**

* [Gorilla Websocket Package](https://pkg.go.dev/github.com/gorilla/websocket)



## Setup

On Windows, Linux macOS, and FreeBSD you will be able to download the pre-built binaries once available.
If your system has [Go >= 1.16+](https://golang.org/dl/) you can pull the codebase and build from the source.


* Build the worker program on windows
```shell
$ git clone https://github.com/jeamon/web-based-jobs-worker-service.git
$ cd web-based-jobs-worker-service
$ go build -o worker.exe .
```

* Build the worker program on linux and others
```shell
$ git clone https://github.com/jeamon/web-based-jobs-worker-service.git
$ cd web-based-jobs-worker-service
$ go build -o worker .
$ chmod +x ./worker
```

* Build with docker for linux-based systems
```shell
$ git clone https://github.com/jeamon/web-based-jobs-worker-service.git
$ cd web-based-jobs-worker-service
$ docker build --tag unix-worker .
$ docker run -d --publish 8080:8080 --name unix-worker --rm unix-worker /bin/sh -c "/app/worker start && sleep infinity"
```



## Get-Started


*Once you have build the worker service executable file - use below available options to control*


* Find below command line options dedicated to the worker service :

	[+] On Windows Operating System.
	```
	worker.exe [ start | stop | restart | status | help | version]
	```

	[+] On Linux Operating System.
	```
	./worker [ start | stop | restart | status | help | version]
	```



## Web-Access


*Once the worker service has started - the embedded web server can handle below requests*

	
* To view this documentation online - for web access or apis calls:
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/docs
	https://<server-ip-address>:<port>/worker/api/v1/docs
	```
	
	[+] From your web browser.
	```
	example: https://127.0.0.1:8080/worker/web/v1/docs
	example: https://127.0.0.1:8080/worker/api/v1/docs
	```


* To execute a quick remote command (optionally with timeout in secs) and get the realtime output:
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/cmd/execute?cmd=<command+argument>
	```
	
	[+] On Windows Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/cmd/execute?cmd=systeminfo
	example: https://127.0.0.1:8080/worker/web/v1/cmd/execute?cmd=ipconfig+/all
	```

	[+] On Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/cmd/execute?cmd=ip+a
	example: https://127.0.0.1:8080/worker/web/v1/cmd/execute?cmd=ps
	```


* To execute a long running remote task (optionally with timeout in mins and dumping to file) and get the realtime output:
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/long/stream/schedule?cmd=<command+argument>&timeout=<value>&dump=<true|false>
	```
	
	[+] On Windows Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=ping+127.0.0.1+-t&dump=true
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=netstat+-an+|+findstr+ESTAB&timeout=60
	```

	[+] On Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=ping+127.0.0.1&dump=true
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=top&timeout=10&dump=true
	```


* To execute multiple long running remote task (optionally with timeout in mins and stream page style) and use their ids to stream their realtime outputs:

*Regarding the style (CSS), the default foreground/background color is white/black. The default size is 18 pixel. If you want to use hex code of a given color,
you must replace the (#) by its encoded value (%23). For example, to use #77216F as background you must fill bg=%2377216F*
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/long/stream/schedule?cmd=<command+argument>&cmd=<command+argument>&timeout=<value>
	&fg=<foreground-color>&bg=<background-color>&bold=[true|false]&size=<value>
	```
	
	[+] On Windows Operating System.
	```
	example: https://localhost:8080/worker/web/v1/jobs/long/stream/schedule?cmd=ping+8.8.8.8+-n+500&fg=white&bold=true&size=20&bg=%2377216F
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=ping+4.4.4.4+-t&cmd=ping+8.8.8.8+-t&timeout=30
	```

	[+] On Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=ping+4.4.4.4&cmd=ping+8.8.8.8&cmd=ping+1.1.1.1&timeout=30
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=&cmd=tail+-f/var/log/syslog&cmd=tail+-f+/var/log/messages&timeout=30
	```


* To stream the output of a single long running job by its id (optionally with specific foreground/background colors name and font size and bold characters):
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/long/output/stream?id=<job-id>&fg=<foreground-color>&bg=<background-color>&bold=[true|false]&size=<value>
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/output/stream?id=abe478954cef4125&size=20&bold=false
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/output/stream?id=abe478954cef4125&fg=gree&bg=black&bold=true
	```


* To execute one or multiple short running jobs (optionally with timeout in seconds) and later use their ids to fetch their outputs:
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/short/schedule?cmd=<command+argument>&cmd=<command+argument>&timeout=<value>
	```

	[+] On Windows Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/short/schedule?cmd=systeminfo&cmd=ipconfig+/all&cmd=tasklist
	example: https://127.0.0.1:8080/worker/web/v1/jobs/short/schedule?cmd=ipconfig+/all
	```

	[+] On Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/short/schedule?cmd=ls+-la&cmd=ip+a&cmd=ps
	```


* To fetch the output of a single short running job by its id:
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/short/output/fetch?id=<job-id>
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/short/output/fetch?id=abe478954cef4125
	```


* To check the detailed status of one or multiple submitted jobs:
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/status/check?id=<job-1-id>&id=<job-2-id>
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/status/check?id=abe478954cef4125&id=cde478910cef4125
	```


* To check the status of all (short and long running) submitted jobs:
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/stop/all?order=[asc|desc]
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/status/check/all
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/status/check/all?order=asc
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/status/check/all?order=desc
	```
	

* To stop one or multiple submitted and running jobs:
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/stop?id=<job-1-id>&id=<job-2-id>
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/stop?id=abe478954cef4125&id=cde478910cef4125
	```

	
* To stop of all (short and long) submitted running jobs:
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/stop/all
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/stop/all
	```


* To restart one or multiple submitted jobs:
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/restart?id=<job-1-id>&id=<job-2-id>
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/restart?id=abe478954cef4125&id=cde478910cef4125
	```


* To restart all (only short running) submitted jobs:
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/restart/all
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/restart/all
	```


* To download the file containing the output of a job (only for long jobs with dump option enabled):
	
	```
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/output/download?id=<job-1-id>
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/output/download?id=cde478910cef4125
	```



## REST-APIs


*Once the worker service has started - the embedded API gateway can handle below calls*


* To execute one or multiple short or long running jobs:
	
	```
	POST /worker/api/v1/jobs/schedule/
    Content-Type: application/json
	Payload : {"jobs":[{job-parameters},...{job-parameters}]} - List of jobs with below fields.
	Parameters:
		<task> 		String 	[Required] full command or task syntax to run.
		<islong> 	Boolean [Required] specifies either it is a short or long job.
		<stream> 	Boolean [Required] when a long job, either to stream over websocket or not.
		<dump> 		Boolean [Required] when a long job, either to stream to disk file or not.
		<memlimit> 	Integer [Optional] maximum memory size to allocate to the process in MB.
		<cpulimit> 	Integer [Optional] maximum cpu to allow to the process in percentage.
		<timeout> 	Integer [Optional] maximum time to process. secs for short and mins for long.
	```
	
	[+] Example Request Payload.
	
	```json
	
	URL: https://127.0.0.1:8080/worker/api/v1/jobs/schedule/
	
	{
		"jobs":[
			{"task":"ping 8.8.8.8", "islong":false, "stream":false, "dump":false, "memlimit": 5, "cpulimit": 1, "timeout": 2},
			{"task":"ping 4.4.4.4", "islong":true, "stream":true, "dump":true, "memlimit": 5, "cpulimit": 1, "timeout": 2}
			{"task":"ping 8.8.8.8", "islong":true, "stream":false, "dump":true, "memlimit": 5, "cpulimit": 1, "timeout": 60}
		]
	}
		
	```
	
	[+] Response payload contains for each job, its parameters with the links to check its status and fetch/stream/download its output.
	
	```json
	
	{
		"requestid": "b24d8634709c2339",
		"jobs": [
			{
				"id": "cd271d1c7077b6be",
				....,
				"statuslink": "https://127.0.0.1:8080/worker/api/v1/jobs/status?id=cd271d1c7077b6be",
				"outputlink": "https://127.0.0.1:8080/worker/api/v1/jobs/fetch?id=cd271d1c7077b6be"
			},
			{
				"id": "c16696a11b12bc06",
				....,
				"statuslink": "https://127.0.0.1:8080/worker/api/v1/jobs/status?id=c16696a11b12bc06",
				"outputlink": "https://127.0.0.1:8080/worker/web/v1/jobs/long/output/stream?id=c16696a11b12bc06"
			},
			{
				"id": "fc1399949df1b477",
				....,
				"statuslink": "https://127.0.0.1:8080/worker/api/v1/jobs/status?id=fc1399949df1b477",
				"outputlink": "https://127.0.0.1:8080/worker/web/v1/jobs/x/output/download?id=fc1399949df1b477"
			}
		]
	}
	```

* To check the status of one or multiple short or long running jobs:

	```
	GET /worker/api/v1/jobs/status?id=<jobID>&id=<jobID>&id=<jobID>&id=<jobID>
	Parameters:
		<jobID> 	String 	[Required] valid 16 hexa characters identifying a job.
	Response contains a requestid and a list of these jobs full information.
	```



* To check the status of all (short and long running or stopped) jobs:

	```
	GET /worker/api/v1/jobs/status/
	Response contains a requestid and a list of all jobs full information.
	```

* To check the liveness of worker service or the availability of API or Web routes:

	```
	GET /worker/ping
	GET /worker/web/ping
	GET /worker/api/ping
	```


* To check get the complete current and in use settings of the worker service:

	```
	GET /worker/settings
	```

## Upcomings

* add filter option to display details of only completed or stopped or running jobs.
* add settings option to store jobs result to redis server or defautl in-memory map.
* add command line options on worker service to list or delete jobs or dump jobs output.
* add feature to move worker service into maintenance mode - stop accepting jobs.
* limit the overall number of jobs scheduling to 10K and make it dynamically configurable.
* refactore logging format by using external library logrus/zerolog for logging into json format.
* enforce puggeable auth/authz with RBAC and add default root user/password at startup.
* refactor job with streaming capability to stream over multiple websockets at the same time.
* display submit/start/end per client browser timezone and send them as unixnano in api response.
* propagate web request or api call id into jobs processing logs.
* enforce per process resources limitations (cpu and memory usage).


## Contribution

Pull requests are welcome. However, I would be glad to be contacted for discussion before.


## License

Please check & read [the license details](https://github.com/jeamon/web-based-jobs-worker/blob/master/LICENSE) or [reach out to me](https://blog.cloudmentor-scale.com/contact) before any action.