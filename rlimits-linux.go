//go:build linux || freebsd
// +build linux freebsd

package main

// @rlimits-linux.go contains function to enforce the maximum number of file descriptors
// that may hold open during execution on linux. This max value is defined into settings
// with field <LinuxWorkerMaxOpenFiles>. Please, note that is a best effort function so
// the program will not stop if it fails to enforce this resource limitation.

import (
	"log"
	"syscall"
)

// enforceMaxOpenFiles ensures that settings value <LinuxWorkerMaxOpenFiles> is enforced.
func enforceMaxOpenFiles() {
	// try to enforce maximum number of files which could be kept open to bypass any default
	// limitations (1024 or 4096) and proactively try avoiding too much too many open files.
	lim := syscall.Rlimit{}
	syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim)
	log.Printf("system default number of open files [cur: %v, max: %v]\n", lim.Cur, lim.Max)

	if lim.Cur < Config.LinuxWorkerMaxOpenFiles || lim.Max < Config.LinuxWorkerMaxOpenFiles {

		if lim.Cur < Config.LinuxWorkerMaxOpenFiles {
			lim.Cur = Config.LinuxWorkerMaxOpenFiles
		}

		if lim.Max < Config.LinuxWorkerMaxOpenFiles {
			lim.Max = Config.LinuxWorkerMaxOpenFiles
		}

		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
			log.Printf("failed to enforce linux max number of open files - errmsg: %v\n", err)
		} else {
			log.Printf("enforced the number of open files to [cur: %v, max: %v]\n", lim.Cur, lim.Max)
		}
	}
}
