# web-based-jobs-worker-service

A go-based fast backend service to spin up and control (stop or fetch or restart or delete) multiple remote jobs (applications or shell commands)
with the capabilities to specify the execution timeout and resources limitations for these jobs. You perform everything just from your web browser. 

* General Features Overview - Click to watch the live [demo video](https://youtu.be/_xnINhN8MRI)
* Dual Streaming Feature - Click to watch the live [demo video](https://youtu.be/NsM7AhWkCko)


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

You must launch the worker service by specifying an option. The available options are start - stop - restart. Once the worker started, it saves its process id (pid)
into a local file named deamon(.)pid since this pid value will be used later to kill the process when you request to stop the worker service. The worker service will
spin up some background procedures (goroutines) such as a monitoring service (cleanupMapResults - to cleanup after a certain time some completed jobs), a signal handler
service (handleSignal - to process any system call exit signals) and a monitoring jobs queue service (jobsMonitor - to watch the jobs queue/channel and start processing).

After creating the dedicated logs folder and files it loads its self-signed certificate and rsa-based private key. In case these files are not present into the certs
folder, they will be created automatically with builtin parameters. This certificate is used by the web service to setup its TLS configurations.

Once the web server is up & running, you can submit one or multiple jobs for remote execution. For each job, a unique id (16 hexadecimal characters) will be generated to
identify this job, so it can be used to check the stop status or stop or restart or fetch the output restult of that job. Each job output is made available into memory
for fast access.

All jobs could be stopped or restarted as well. When a job restart is requested, in case this job is still running, it will only be stopped and you will need to restart it
again in order to have it scheduled for processing. If already completed it will be immediately scheduled for processing.

Finally, you can submit a single job and wait until it completes to view its result output immediately. Or you can submit a long running job and view in realtime (in streaming)
it log output. Each start of the worker service creates three logs files. One file logs all web requests, another for each job processing details and the third one for recording
each job details deleted by the cleanup background routine. The worker itself logs its standard output and standard error into a single file named worker(.)log. You can view all
the features APIs exposed by the web server under the [usage section](#usage).


## Features

Below is a summary of available features. This section will be updated as project continue:

* command-line options to start or stop or restart the worker
* command-line option to check the status of the worker service
* submit one or several jobs from browser or any web client 
* check the status of one or several jobs based on their ids
* request the stop of one or several jobs based on their ids
* fetch the execution result of a singlejob based on its id
* request the status or the stop of all submitted jobs
* unique output log folder for each worker startup
* single fixed worker log for standard output & error
* per startup web server log and jobs log and deletion cron log 
* https-only local web server with auto-generated self-signed certificate 
* capability to restart one or more jobs by their ids or restart all jobs
* schedule a long running task and start viewing the job output in realtime
* schedule multiple long running jobs and use their id for live streaming their output
* schedule a long running task with dual streaming (to a disk file and over websocket)


Please feel free to have a look at the [usage section](#usage) for examples.



## Technologies

This project is developed with:
* Golang version: 1.13+
* Golang Native packages
* [Gorilla Websocket Package](https://pkg.go.dev/github.com/gorilla/websocket)


## Setup

On Windows, Linux macOS, and FreeBSD you will be able to download the pre-built binaries once available.
If your system has [Go >= 1.13+](https://golang.org/dl/) you can pull the codebase and build from the source.


* Build the worker program on windows
```shell
$ git clone https://github.com/jeamon/wweb-based-jobs-worker-service.git
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


## Usage


* Find below command line options dedicated to the worker service :

	[+] On Windows Operating System.
	```
	worker.exe [ start | stop | restart | status | help | version]
	```

	[+] On Linux Operating System.
	```
	./worker [ start | stop | restart | status | help | version]
	```

	
* To execute a quick remote command (optionally with timeout in secs) and get the realtime output:
	
	```
	https://<server-ip-address>:<port>/worker/v1/cmd/execute?cmd=<command+argument>
	```
	
	[+] On Windows Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/cmd/execute?cmd=systeminfo
	example: https://127.0.0.1:8080/worker/v1/cmd/execute?cmd=ipconfig+/all
	example: https://127.0.0.1:8080/worker/v1/cmd/execute?cmd=netstat+-an+|+findstr+ESTAB&timeout=45
	```

	[+] On Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/cmd/execute?cmd=ls+-la
	example: https://127.0.0.1:8080/worker/v1/cmd/execute?cmd=ip+a
	example: https://127.0.0.1:8080/worker/v1/cmd/execute?cmd=ps
	```


* To execute a long running remote task (optionally with timeout in mins and dumping to file) and get the realtime output:
	
	```
	https://<server-ip-address>:<port>/worker/v1/jobs/long/stream/schedule?cmd=<command+argument>&timeout=<value>&dump=<true|false>
	```
	
	[+] On Windows Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/long/stream/schedule?cmd=ping+127.0.0.1+-t&dump=true
	example: https://127.0.0.1:8080/worker/v1/jobs/long/stream/schedule?cmd=netstat+-an+|+findstr+ESTAB&timeout=60
	```

	[+] On Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/long/stream/schedule?cmd=ping+127.0.0.1&dump=true
	example: https://127.0.0.1:8080/worker/v1/jobs/long/stream/schedule?cmd=top&timeout=10&dump=true
	```


* To execute multiple long running remote task (optionally with timeout in mins) and use their ids to stream their realtime outputs:
	
	```
	https://<server-ip-address>:<port>/worker/v1/jobs/long/stream/schedule?cmd=<command+argument>&cmd=<command+argument>&timeout=<value>
	```
	
	[+] On Windows Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/long/stream/schedule?cmd=ping+4.4.4.4+-t&cmd=ping+8.8.8.8+-t&timeout=30
	example: https://127.0.0.1:8080/worker/v1/jobs/long/stream/schedule?cmd=netstat+-an+|+findstr+ESTAB&netstat+-an+|+findstr+ESTAB&timeout=15
	```

	[+] On Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/long/stream/schedule?cmd=ping+4.4.4.4&cmd=ping+8.8.8.8&cmd=ping+1.1.1.1&timeout=30
	example: https://127.0.0.1:8080/worker/v1/jobs/long/stream/schedule?cmd=&cmd=tail+-f/var/log/syslog&cmd=tail+-f+/var/log/messages&timeout=30
	```


* To execute one or multiple short running jobs (optionally with timeout in seconds) and later use their ids to fetch their outputs:
	
	```
	https://<server-ip-address>:<port>/worker/v1/jobs/short/schedule?cmd=<command+argument>&cmd=<command+argument>&timeout=<value>
	```

	[+] On Windows Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/short/schedule?cmd=systeminfo&cmd=ipconfig+/all&cmd=tasklist
	example: https://127.0.0.1:8080/worker/v1/jobs/short/schedule?cmd=ipconfig+/all
	```

	[+] On Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/short/schedule?cmd=ls+-la&cmd=ip+a&cmd=ps
	```


* To check the detailed status of one or multiple submitted jobs:
	
	```
	https://<server-ip-address>:<port>/worker/v1/jobs/x/status/check?id=<job-1-id>&id=<job-2-id>
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/x/status/check?id=abe478954cef4125&id=cde478910cef4125
	```


* To check the status of all (short and long running) submitted jobs:
	
	```
	https://<server-ip-address>:<port>/worker/v1/jobs/x/stop/all?order=[asc|desc]
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/x/status/check/all
	example: https://127.0.0.1:8080/worker/v1/jobs/x/status/check/all?order=asc
	example: https://127.0.0.1:8080/worker/v1/jobs/x/status/check/all?order=desc
	```
	

* To fetch the output of a single short running job by its id:
	
	```
	https://<server-ip-address>:<port>/worker/v1/jobs/short/output/fetch?id=<job-id>
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/short/output/fetch?id=abe478954cef4125
	```


* To stop one or multiple submitted and running jobs:
	
	```
	https://<server-ip-address>:<port>/worker/v1/jobs/x/stop?id=<job-1-id>&id=<job-2-id>
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/x/stop?id=abe478954cef4125&id=cde478910cef4125
	```

	
* To stop of all (short and long) submitted running jobs:
	
	```
	https://<server-ip-address>:<port>/worker/v1/jobs/x/stop/all
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/x/stop/all
	```


* To restart one or multiple submitted jobs:
	
	```
	https://<server-ip-address>:<port>/worker/v1/jobs/x/restart?id=<job-1-id>&id=<job-2-id>
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/x/restart?id=abe478954cef4125&id=cde478910cef4125
	```


* To restart all (only short running) submitted jobs:
	
	```
	https://<server-ip-address>:<port>/worker/v1/jobs/x/restart/all
	```

	[+] On Windows or Linux Operating System.
	```
	example: https://127.0.0.1:8080/worker/v1/jobs/x/restart/all
	```



## Upcomings

* add filter option to display details of only completed or stopped or running jobs
* add capability to load configuration details from a file at start time
* add option to store jobs result to redis server rather than in-memory map
* add URI and handler to download execution output of one or multiple jobs by ids 
* add command line options on worker service to list or delete jobs or dump jobs output
* add feature to move worker service into maintenance mode - stop accepting jobs
* add URI & handler to schedule multiple long running jobs and stream their output to files
* identify each web request with a unique id for further tracing using middleware & http context
* limit the overall number of jobs scheduling to 10K and make it dynamically configurable
* embed websocket html/JS template file into executable by leveraging golang 1.16 feature
* dump dead jobs output to disk file before removing them from the result queue - cleanupMapResults()
* embed a RestFul API service to schedule - status check - stop/restart - fetch jobs : /worker/<api>/v1/



## Contribution

Pull requests are welcome. However, I would be glad to be contacted for discussion before.


## License

please check & read [the license details](https://github.com/jeamon/web-based-jobs-worker/blob/master/LICENSE) or [reach out to me](https://blog.cloudmentor-scale.com/contact) before any action.