package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// build a string made of dash symbol - used to display table.
func Dashs(count int) string {
	return strings.Repeat("-", count)
}

// PathExists returns whether the given file or directory exists.
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsDir returns whether the given path is a directory.
func IsDir(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return stat.IsDir(), nil
}

// truncateSyntax takes a command syntax as input and returns a shorten version of this syntax
// while taking into account the maximun number of characters.
func truncateSyntax(syntax string, maxlength int) string {
	if utf8.RuneCountInString(syntax) > maxlength {
		r := []rune(syntax)
		syntax = string(r[:maxlength])
	}
	return syntax
}

// formatSize takes the size of a bytes buffer or file in float64 and converts to KB
// then formats it with 0 - 4 digits after the point depending of the value.
func formatSize(size float64) string {
	size = size / 1024
	if size < 10.0 {
		return fmt.Sprintf("%.4f", size)
	} else if size < 100.0 {
		return fmt.Sprintf("%.3f", size)
	} else if size < 1000.0 {
		return fmt.Sprintf("%.2f", size)
	} else if size < 10000.0 {
		return fmt.Sprintf("%.1f", size)
	} else {
		return fmt.Sprintf("%.0f", size)
	}
}

// generateID uses rand from crypto module to generate random
// value into hexadecimal mode this value will be used as job id
func generateID() string {

	// randomly fill the 8 capacity slice of bytes
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// use current number of nanoseconds since January 1, 1970 UTC
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

// createFolder makes sure that <folderPath> is present, and if not creates it.
func createFolder(folderPath string) {
	ok, err := PathExists(folderPath)
	if ok {
		if ok, err = IsDir(folderPath); ok && err == nil {
			return
		} else {
			log.Printf("path %q exists but it is not a folder so please check before continue - errmsg : %v\n", folderPath, err)
			os.Remove(Config.WorkerPidFilePath)
			os.Exit(1)
		}
	}
	// try to create the folder.
	err = os.MkdirAll(folderPath, 0755)
	if err != nil {
		log.Printf("failed to create %q folder - errmsg : %v\n", folderPath, err)
		// try to remove any PID file.
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(1)
	}
}

// resetCompletedJobInfos resets a given job details (only if it has been completed/stopped before) for restarting.
func resetCompletedJobInfos(j *Job) {
	j.pid = 0
	j.iscompleted, j.issuccess = false, false
	j.fetchcount = 0
	j.isstreaming = false
	j.exitcode = -1
	j.errormsg = ""
	j.starttime, j.endtime = time.Time{}, time.Time{}
	(j.result).Reset()
}

// removeDuplicateJobIds rebuilds the slice of job ids (string type) by verifying the format and deleting
// duplicate elements. In case there is no remaining valid id it returns true to ignore the request.
func removeDuplicateJobIds(ids *[]string) bool {

	if len(*ids) == 1 {
		if match, _ := regexp.MatchString(`[a-z0-9]{16}`, (*ids)[0]); !match {
			return true
		}
	}

	temp := make(map[string]struct{})
	for _, id := range *ids {
		if match, _ := regexp.MatchString(`[a-z0-9]{16}`, id); !match {
			continue
		}
		temp[id] = struct{}{}
	}
	*ids = nil
	*ids = make([]string, 0)
	for id, _ := range temp {
		*ids = append(*ids, id)
	}

	if len(*ids) == 0 {
		return true
	}

	return false
}
