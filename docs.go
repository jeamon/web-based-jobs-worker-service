package main

// @docs.go contains the how to use this worker service. which is accessible from the root web interface.

const infos = "web-based <jobs-worker-service>"

var (
	Version    string
	BuildTime  string
	GitCommit  string
	GoVersion  string
	TargetOS   string
	TargetArch string
	WebVersion string
	APIVersion string
	Author     string = "Jerome Amon <cloudmentor.scale@gmail.com>"
	SourceLink string = "https://github.com/jeamon/web-based-jobs-worker-service/commit/"
)

const webv1docs = `


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
	
			   [latest version By Jerome AMON - cloudmentor.scale@gmail.com]

	----------------------------------------------------------------------------------------------


	[00] Find below command line options dedicated to the worker service :

	[+] On Windows Operating System.
	worker.exe [ start | stop | restart | status | help | version]

	[+] On Linux Operating System.
	./worker [ start | stop | restart | status | help | version]

	----------
	
	[01] Execute a quick remote command (optionally with timeout in secs) and get the realtime output:
	
	https://<server-ip-address>:<port>/worker/web/v1/cmd/execute?cmd=<command+argument>
	
	[+] On Windows Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/cmd/execute?cmd=systeminfo
	example: https://127.0.0.1:8080/worker/web/v1/cmd/execute?cmd=ipconfig+/all
	example: https://127.0.0.1:8080/worker/web/v1/cmd/execute?cmd=netstat+-an+|+findstr+ESTAB&timeout=45

	[+] On Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/cmd/execute?cmd=ls+-la
	example: https://127.0.0.1:8080/worker/web/v1/cmd/execute?cmd=ip+a
	example: https://127.0.0.1:8080/worker/web/v1/cmd/execute?cmd=ps

	----------

	[02] Execute a long running remote job (with timeout in mins and dumping to file) and stream its output:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/long/stream/schedule?cmd=<command+argument>&timeout=<value>&dump=<true|false>

	[+] On Windows Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=ping+127.0.0.1+-t&dump=true
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=netstat+-an+|+findstr+ESTAB&timeout=60

	[+] On Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=ping+127.0.0.1&dump=true
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=top&timeout=10&dump=true

	----------

	[03] Execute multiple long running remote jobs (with timeout in mins) and use their ids to stream their realtime outputs:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/long/stream/schedule?cmd=<command+argument>&cmd=<command+argument>&timeout=<value>

	[+] On Windows Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=ping+4.4.4.4+-t&cmd=ping+8.8.8.8+-t&timeout=30
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=netstat+-an+|+findstr+ESTAB&netstat+-an+|+findstr+ESTAB&timeout=15

	[+] On Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=ping+4.4.4.4&cmd=ping+8.8.8.8&cmd=ping+1.1.1.1&timeout=30
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/stream/schedule?cmd=&cmd=tail+-f/var/log/syslog&cmd=tail+-f+/var/log/messages&timeout=30
	
	----------

	[04] To stream the output of a single long running job by its id:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/long/output/stream?id=<job-id>

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/long/output/stream?id=abe478954cef4125

	----------

	[05] Execute one or multiple short running jobs (optionally with timeout in seconds) and later use their ids to fetch their outputs:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/short/schedule?cmd=<command+argument>&cmd=<command+argument>&timeout=<value>

	[+] On Windows Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/short/schedule?cmd=systeminfo&cmd=ipconfig+/all&cmd=tasklist
	example: https://127.0.0.1:8080/worker/web/v1/jobs/short/schedule?cmd=ipconfig+/all

	[+] On Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/short/schedule?cmd=ls+-la&cmd=ip+a&cmd=ps

	----------

	[06] To fetch the output of a single short running job by its id:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/short/output/fetch?id=<job-id>

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/short/output/fetch?id=abe478954cef4125

	----------

	[07] To check the detailed status of one or multiple submitted jobs:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/status/check?id=<job-1-id>&id=<job-2-id>

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/status/check?id=abe478954cef4125&id=cde478910cef4125

	----------

	[08] To check the status of all (short and long running) submitted jobs:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/stop/all?order=[asc|desc]

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/status/check/all
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/status/check/all?order=asc
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/status/check/all?order=desc

	----------

	[09] To fetch the output of a single short running job by its id:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/short/output/fetch?id=<job-id>

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/short/output/fetch?id=abe478954cef4125

	----------

	[10] To stop one or multiple submitted and running jobs:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/stop?id=<job-1-id>&id=<job-2-id>

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/stop?id=abe478954cef4125&id=cde478910cef4125

	----------

	[11] To stop of all (short and long) submitted running jobs:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/stop/all

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/stop/all

	----------

	[12] To restart one or multiple submitted jobs:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/restart?id=<job-1-id>&id=<job-2-id>

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/restart?id=abe478954cef4125&id=cde478910cef4125

	----------

	[13] To restart all (only short running) submitted jobs:
	
	https://<server-ip-address>:<port>/worker/web/v1/jobs/x/restart/all

	[+] On Windows or Linux Operating System.
	example: https://127.0.0.1:8080/worker/web/v1/jobs/x/restart/all

	----------

	`
