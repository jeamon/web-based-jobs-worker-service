package main

const version = "This tool is <jobs-worker-service> • version 1.0 By Jerome AMON"

const help = `


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

	[6] To stop of one or more submitted running commands (jobs):
	
	http://<server-ip-address>:<port>/jobs/stop?id=<job-1-id>&id=<job-2-id>

	[+] On Windows or Linux Operating System.
	example: http://127.0.0.1:8080/jobs/stop?id=abe478954cef4125&id=cde478910cef4125

	----------------------------------------------------------------------------------------------

	[7] To stop of all submitted running commands (jobs):
	
	http://<server-ip-address>:<port>/jobs/stop/

	[+] On Windows or Linux Operating System.
	example: http://127.0.0.1:8080/jobs/stop/

	----------------------------------------------------------------------------------------------

	`
