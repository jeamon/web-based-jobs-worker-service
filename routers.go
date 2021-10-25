package main

// @routers.go : contains all web routes and API URIs with their respective handlers functions.

import (
	"net/http"
)

// setupWebServerRoutes configures web access routes.
func setupWebServerRoutes(router *http.ServeMux) {
	// URI to check web & api routes availabilities.
	if Config.EnableWebAccess {
		router.HandleFunc("/worker/ping", pong)
	}

	// URI to check WEB routes availabilities.
	router.HandleFunc("/worker/web/ping", webPong)

	// default web request - send web documentation.
	router.HandleFunc("/", webV1Help)
	router.HandleFunc("/worker/web/v1/docs", webV1Help)

	// expected request : /worker/web/v1/cmd/execute?cmd=<task-syntax>&timeout=<value>
	router.HandleFunc("/worker/web/v1/cmd/execute", instantCommandExecutor)
	// expected request : /worker/web/v1/jobs/short/schedule?cmd=<task-syntax>
	router.HandleFunc("/worker/web/v1/jobs/short/schedule", scheduleShortRunningJobs)

	// expected request : /worker/web/v1/jobs/x/status/check?id=<jobid>
	router.HandleFunc("/worker/web/v1/jobs/x/status/check", checkJobsStatusById)
	// expected request : /worker/web/v1/jobs/x/status/check/all?order=asc|desc
	router.HandleFunc("/worker/web/v1/jobs/x/status/check/all", getAllJobsStatus)

	// expected format : /worker/web/v1/jobs/short/output/fetch?id=<jobid>
	router.HandleFunc("/worker/web/v1/jobs/short/output/fetch", getJobsOutputById)

	// expected request : /worker/web/v1/jobs/x/stop?id=<jobid>&id=<jobid>
	router.HandleFunc("/worker/web/v1/jobs/x/stop", stopJobsById)
	// expected request : /worker/web/v1/jobs/x/stop/all
	router.HandleFunc("/worker/web/v1/jobs/x/stop/all", stopAllJobs)

	// expected request : /worker/web/v1/jobs/x/restart?id=<jobid>&id=<jobid>
	router.HandleFunc("/worker/web/v1/jobs/x/restart", restartJobsById)
	// expected request : /worker/web/v1/jobs/x/restart/all
	router.HandleFunc("/worker/web/v1/jobs/x/restart/all", restartAllJobs)

	// live streaming a long running job output: /worker/web/v1/jobs/long/output/stream?id=<jobid>
	// optionally add &fg=<text-color>&bg=<page-color>&bold=<true|false>&size=<font-size-in-pixels>
	router.HandleFunc("/worker/web/v1/jobs/long/output/stream", streamJobsOutputById)
	// download a job output file: /worker/web/v1/jobs/x/output/download?id=<jobid>
	router.HandleFunc("/worker/web/v1/jobs/x/output/download", downloadJobsOutputById)

	// schedule a long running job with output streaming capability.
	// /worker/web/v1/jobs/long/stream/schedule?cmd=<task>&cmd=<task>&timeout=<value>&save=true|false
	// optionally add &fg=<text-color>&bg=<page-color>&bold=<true|false>&size=<font-size-in-pixels>
	router.HandleFunc("/worker/web/v1/jobs/long/stream/schedule", scheduleLongJobsWithStreaming)
	// schedule a long running job with only streaming output to disk file.
	// /worker/web/v1/jobs/long/dump/schedule?cmd=<task>&cmd=<task>&timeout=<value>
	router.HandleFunc("/worker/web/v1/jobs/long/dump/schedule", scheduleLongJobsWithDumping)
}

// setupApiGatewayRoutes configures apis routes.
func setupApiGatewayRoutes(router *http.ServeMux) {
	// URI to check API routes availabilities.
	router.HandleFunc("/worker/api/ping", apiPong)
	// URI to check web & api routes availabilities.
	// Enabled if web routes are disabled.
	if !Config.EnableWebAccess {
		router.HandleFunc("/worker/ping", pong)
	}

	// default api request - send apis documentation.
	// router.HandleFunc("/worker/api/v1/docs", webV1Help)

	// expected request : POST /worker/api/v1/jobs/schedule/
	router.HandleFunc("/worker/api/v1/jobs/schedule/", apiScheduleJobs)

	// expected request : GET /worker/api/v1/jobs/status?id=<jobid>
	router.HandleFunc("/worker/api/v1/jobs/status", apiCheckJobsStatusById)
	// expected request : GET /worker/api/v1/jobs/status/
	router.HandleFunc("/worker/api/v1/jobs/status/", apiCheckAllJobsStatus)

	// expected request : GET /worker/api/v1/jobs/fetch?id=<jobid>
	router.HandleFunc("/worker/api/v1/jobs/fetch", apiFetchJobsOutputById)

	// expected request : GET /worker/api/v1/jobs/stop?id=<jobid>
	//router.HandleFunc("/worker/api/v1/jobs/stop", apiStopJobsById)
	// expected request : GET /worker/api/v1/jobs/stop/
	//router.HandleFunc("/worker/api/v1/jobs/stop/", apiStopAllJobs)
}
