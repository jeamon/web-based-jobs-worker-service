package main

// @configuration.go : contains the data structures which stores the default worker configuration and
// the mandatory settings options, and function to load them from the config file <worker.config.json>.

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

// Configuation is the structure of the expected json data to be loaded from the config file <worker.config.json>.
type Configuration struct {
	HttpsServerHost                  string `json:"https_server_host"`                    // https server ip or dns.
	HttpsServerPort                  string `json:"https_server_port"`                    // https server port number.
	HttpsServerCerts                 string `json:"https_server_certs"`                   // https server certificate filename.
	HttpsServerKey                   string `json:"https_server_key"`                     // https server private key filename.
	HttpsServerCertsPath             string `json:"https_server_certs_path"`              // where to find https server certs/key.
	HttpsServerCertsEmail            string `json:"https_server_certs_email"`             // email to use when building server certs.
	WorkerPidFilePath                string `json:"worker_pid_file_path"`                 // worker service process identifier file.
	WorkerLogFilePath                string `json:"worker_log_file_path"`                 // place to store worker process outputs.
	LogFoldersLocation               string `json:"log_folders_location"`                 // folder to store daily startup logs.
	WebRequestsLogFile               string `json:"web_requests_log_file"`                // file to create and storing all web requests.
	ApiRequestsLogFile               string `json:"api_requests_log_file"`                // file to create for storing all api calls.
	JobsProcessLogFile               string `json:"jobs_process_log_file"`                // file to create for storing jobs processing lifecycle.
	JobsDeletionLogFile              string `json:"jobs_deletion_log_file"`               // file to create for tracking auto-deleted jobs details.
	DefaultLinuxShell                string `json:"default_linux_shell"`                  // shell path to be used on linux platform as last resort.
	JobsOutputsFolder                string `json:"jobs_outputs_folder"`                  // folder to create for storing long jobs outputs dump.
	WorkerWorkingDirectory           string `json:"worker_working_directory"`             // path to be used as worker process directory.
	JobsCleanupMaxFetch              int    `json:"jobs_cleanup_max_fetch"`               // maximum number of output fetch to allow job deletion.
	JobsCleanupRunInterval           int    `json:"jobs_cleanup_run_interval"`            // each number of hours to check for jobs deletion.
	JobsCleanupMaxAge                int    `json:"jobs_cleanup_max_age"`                 // hours since job ended to consider as dead job to delete.
	WaitTimeBeforeExit               int    `json:"wait_time_before_exit"`                // seconds to pause before exiting from the worker.
	ShortJobTimeout                  int    `json:"short_job_timeout"`                    // default and max timeout in seconds for short running jobs.
	LongJobTimeout                   int    `json:"long_job_timeout"`                     // default and max timeout in minutes for long running jobs.
	MaxJobsQueueBuffer               int    `json:"max_jobs_queue_buffer"`                // maximum jobs to queue at once on channel for pickup.
	PidFileWatchInterval             int    `json:"pid_file_watch_interval"`              // each number of mins to check the pidfile presence before self-shutdown.
	EnableWebAccess                  bool   `json:"enable_web_access"`                    // specifies if web routes should be setup or not.
	EnableAPIGateway                 bool   `json:"enable_api_gateway"`                   // specifies if api routes should be setup or not.
	MaxNumberOfJobs                  int    `json:"max_number_of_jobs"`                   // maximum number of jobs that could be processed.
	OrganizationNameForCerts         string `json:"organization_name_for_certs"`          // Organization name to be used for self-signed CA.
	CountryCodeForCerts              string `json:"country_code_for_certs"`               // Country code to be used for self-signed certificate.
	ProvinceNameForCerts             string `json:"province_name_for_certs"`              // Province name to be used for self-signed certificate.
	CityNameForCerts                 string `json:"city_name_for_certs"`                  // City name to be used for self-signed certificate.
	StreetAddressForCerts            string `json:"street_address_for_certs"`             // Street name to be used for self-signed certificate.
	PostalCodeForCerts               string `json:"postal_code_for_certs"`                // Postal code to be used for self-signed certificate.
	MemoryLimitMaxMegaBytes          int    `json:"memory_limit_max_megabytes"`           // maximum memory usage for any job process in megabytes.
	MemoryLimitDefaultMegaBytes      int    `json:"memory_limit_default_megabytes"`       // default memory usage for any job process in megabytes.
	CpuLimitDefaultPercentage        int    `json:"cpu_limit_default_percentage"`         // default cpu usage for any job process in percentage.
	CpuLimitMaxPercentage            int    `json:"cpu_limit_max_percentage"`             // maximum cpu usage for any job process in percentage.
	StreamPageDefaultForegroundColor string `json:"stream_page_default_foreground_color"` // if text color not set in stream request use this.
	StreamPageDefaultBackgroundColor string `json:"stream_page_default_background_color"` // if page background color not set in stream request use this.
	StreamPageDefaultFontSize        int    `json:"stream_page_default_font_size"`        // if page font size not set in stream request use this.
	JobsOutputsBackupsFolder         string `json:"jobs_outputs_backups_folder"`          // folder to store jobs outputs as backup before cleanup/deletion.
}

// config filename to be used to setup the worker service.
var DefaultConfigFile = "worker.config.json"

// Config is the current configuration in use.
var Config = Configuration{}

// mandatory default worker configuration to be loaded if
// <worker.config.json> file not found at start time.
var defaultConfig = Configuration{
	HttpsServerHost:                  "127.0.0.1",
	HttpsServerPort:                  "8080",
	HttpsServerCerts:                 "server.crt",
	HttpsServerKey:                   "server.key",
	HttpsServerCertsPath:             "./certs",
	HttpsServerCertsEmail:            "",
	WorkerPidFilePath:                "worker.service.pid",
	WorkerLogFilePath:                "worker.log",
	LogFoldersLocation:               "./logs",
	WebRequestsLogFile:               "web.log",
	ApiRequestsLogFile:               "api.log",
	JobsProcessLogFile:               "jobs.log",
	JobsDeletionLogFile:              "deleted.log",
	DefaultLinuxShell:                "/bin/sh",
	JobsOutputsFolder:                "./outputs",
	WorkerWorkingDirectory:           "./",
	JobsCleanupMaxFetch:              10,
	JobsCleanupRunInterval:           12,
	JobsCleanupMaxAge:                24,
	WaitTimeBeforeExit:               3,
	ShortJobTimeout:                  3600,
	LongJobTimeout:                   1440,
	MaxJobsQueueBuffer:               10,
	PidFileWatchInterval:             60,
	EnableWebAccess:                  true,
	EnableAPIGateway:                 true,
	MaxNumberOfJobs:                  10000,
	OrganizationNameForCerts:         "",
	CountryCodeForCerts:              "",
	ProvinceNameForCerts:             "",
	CityNameForCerts:                 "",
	StreetAddressForCerts:            "",
	PostalCodeForCerts:               "",
	MemoryLimitMaxMegaBytes:          1000,
	MemoryLimitDefaultMegaBytes:      100,
	CpuLimitDefaultPercentage:        10,
	CpuLimitMaxPercentage:            25,
	StreamPageDefaultForegroundColor: "white",
	StreamPageDefaultBackgroundColor: "black",
	StreamPageDefaultFontSize:        18,
	JobsOutputsBackupsFolder:         "./backups",
}

// dumpDefaultConfig is triggered when passed config file is not found or erroned. It loads
// the default settings for running the worker and create that file for saving the configs.
func dumpDefaultConfig() error {
	log.Printf("loading with default settings. dumped to <%s>.\n", DefaultConfigFile)
	Config = defaultConfig
	data, err := json.MarshalIndent(defaultConfig, "", "\t")
	if err == nil {
		err = ioutil.WriteFile(DefaultConfigFile, data, 0644)
	}
	return err
}

// loadConfigFile loads a JSON based configuration file then initialize the Config variable.
// If the configfile passed does not exist, or an error occurs during the loading it will
// create <worker.config.json> in the current directory and populate it with the DefaultConfig.
func loadConfigFile(configfile string) {
	var err error
	var data []byte
	var ok bool

	ok, err = PathExists(configfile)
	if ok {
		// config file exists, loading the configs.
		data, err = ioutil.ReadFile(configfile)
		if err == nil {
			// file content loaded.
			err = json.Unmarshal(data, &Config)
			if err == nil {
				// valid json format.
				return
			}
		}
	}
	// save default config to file.
	err = dumpDefaultConfig()
	if err != nil {
		log.Printf("failed to load settings and to dump default configs to file - errmsg: %v\n", err)
		os.Exit(1)
	}
}
