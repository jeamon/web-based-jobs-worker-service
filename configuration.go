package main

// @configuration.go : contains the data structures which stores the default worker configuration and
// the mandatory settings options, and function to load them from the config file <worker.config.json>.

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

// config filename to be used to setup the worker service.
var DefaultConfigFile = "worker.config.json"

// Config is the current configuration in use.
var Config = Configuration{}

// mandatory default worker configuration to be loaded if
// <worker.config.json> file not found at start time.
var defaultConfig = Configuration{
	HttpsServerHost:        "127.0.0.1",
	HttpsServerPort:        "8080",
	HttpsServerCerts:       "server.crt",
	HttpsServerKey:         "server.key",
	HttpsServerCertsPath:   "./certs",
	HttpsServerCertsEmail:  "",
	WorkerPidFilePath:      "worker.service.pid",
	WorkerLogFilePath:      "worker.log",
	LogFoldersLocation:     "./logs",
	WebRequestsLogFile:     "web.log",
	ApiRequestsLogFile:     "api.log",
	JobsProcessLogFile:     "jobs.log",
	JobsDeletionLogFile:    "deleted.log",
	DefaultLinuxShell:      "/bin/sh",
	JobsOutputsFolder:      "./outputs",
	WorkerWorkingDirectory: "./",
	JobsCleanupMaxFetch:    10,
	JobsCleanupRunInterval: 12,
	JobsCleanupMaxAge:      24,
	WaitTimeBeforeExit:     3,
	ShortJobTimeout:        3600,
	LongJobTimeout:         1440,
	MaxJobsQueueBuffer:     10,
	PidFileWatchInterval:   60,
	EnableWebAccess:        true,
	EnableAPIGateway:       true,
	MaxNumberOfJobs:        10000,
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
