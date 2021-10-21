package main

// @helpers.go contains useful functions to be used into this project.

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

// getFromRequest retrieves a given key's value from the http request context.
func getFromRequest(r *http.Request, key string) string {
	if value := r.Context().Value(key); value != nil {
		return value.(string)
	}
	return "n/a"
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

// generateID uses rand from crypto module to generate random size value into hexadecimal mode.
// This value is used to build uid for job and request (web &api). For size, it returns (size x 2) chars.
func generateID(size int) string {

	// randomly fill the 8 capacity slice of bytes
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		// use current number of nanoseconds since January 1, 1970 UTC
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

// generateApiRequestID returns a string which is used as API call id.
// The returned string follows this format : API.211019.204450.xxxxxx
func generateApiRequestID(t time.Time) string {
	if Config.EnableLogsTimestampInUTC {
		t = t.UTC()
	}
	return fmt.Sprintf("API.%02d%02d%02d.%02d%02d%02d.%s", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), generateID(3))
}

// generateWebRequestID returns a string which is used as WEB request id.
// The returned string follows this format : WEB.211019.204450.xxxxxx
func generateWebRequestID(t time.Time) string {
	if Config.EnableLogsTimestampInUTC {
		t = t.UTC()
	}
	return fmt.Sprintf("WEB.%02d%02d%02d.%02d%02d%02d.%s", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), generateID(3))
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
func (job *Job) resetCompletedJobInfos() {
	job.lock.Lock()
	job.pid = 0
	job.iscompleted, job.issuccess = false, false
	job.fetchcount = 0
	job.isstreaming = false
	job.exitcode = -1
	job.errormsg = ""
	job.starttime, job.endtime = time.Time{}, time.Time{}
	(job.result).Reset()
	job.lock.Unlock()
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

// SendErrorMessage sends error messages into JSON as HTTP response.
func SendErrorMessage(w http.ResponseWriter, requestid string, message string, code int) {

	errorDetails := ApiErrorMessage{
		Message:   message,
		Code:      code,
		RequestId: requestid,
		Status:    false,
	}

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(errorDetails); err != nil {
		apilog.Printf("[request:%s] failed to send jobs scheduling failure response - errmsg: %v\n", requestid, err)
		return
	}

	apilog.Printf("[request:%s] success to send jobs scheduling failure response\n", requestid)
}

// scheduledInfos Job method to returns essential details once a job is just scheduled.
func (job *Job) scheduledInfos() JobScheduledInfos {
	job.lock.RLock()
	info := JobScheduledInfos{
		Id:         job.id,
		Task:       job.task,
		IsLong:     job.islong,
		MemLimit:   job.memlimit,
		CpuLimit:   job.cpulimit,
		Timeout:    job.timeout,
		Stream:     job.stream,
		Dump:       job.dump,
		SubmitTime: (job.submittime).Format("2006-01-02 15:04:05"),
		StatusLink: fmt.Sprintf("https://%s:%s/worker/api/v1/jobs/status?id=%s", Config.HttpsServerHost, Config.HttpsServerPort, job.id),
	}
	job.lock.RUnlock()

	if job.islong {
		if job.stream {
			// long job with streaming capability.
			info.OutputLink = fmt.Sprintf("https://%s:%s/worker/web/v1/jobs/long/output/stream?id=%s", Config.HttpsServerHost, Config.HttpsServerPort, job.id)
		} else if job.dump {
			// for long job with dump to file only option, we should download the file.
			info.OutputLink = fmt.Sprintf("https://%s:%s/worker/web/v1/jobs/x/output/download?id=%s", Config.HttpsServerHost, Config.HttpsServerPort, job.id)
		} else {
			// long job with no web & file stream options -> /dev/null.
			info.OutputLink = "n/a"
		}
	} else {
		// short job.
		info.OutputLink = fmt.Sprintf("https://%s:%s/worker/api/v1/jobs/fetch?id=%s", Config.HttpsServerHost, Config.HttpsServerPort, job.id)
	}

	return info
}

// collectStatusInfos Job method to returns full status details a job.
func (job *Job) collectStatusInfos() JobStatusInfos {
	var start, end, sizeFormat string
	job.lock.RLock()
	// format time display for zero time values.
	if (job.starttime).IsZero() {
		start = "N/A"
	} else {
		start = (job.starttime).Format("2006-01-02 15:04:05")
	}

	if (job.endtime).IsZero() {
		end = "N/A"
	} else {
		end = (job.endtime).Format("2006-01-02 15:04:05")
	}

	if !job.islong {
		// short job, get memory buffer length.
		sizeFormat = formatSize(float64(job.result.Len()))
	} else if job.islong && job.dump {
		// long job with dumping to file option, use file size.
		fi, err := os.Stat(job.filename)
		if err == nil {
			sizeFormat = formatSize(float64(fi.Size()))
		} else {
			sizeFormat = "N/A"
		}
	} else {
		// long job with dump option.
		sizeFormat = "N/A"
	}

	info := JobStatusInfos{
		Id:          job.id,
		Pid:         job.pid,
		Task:        job.task,
		IsLong:      job.islong,
		Stream:      job.stream,
		Dump:        job.dump,
		IsCompleted: job.iscompleted,
		IsSuccess:   job.issuccess,
		ExitCode:    job.exitcode,
		DataSize:    sizeFormat,
		FetchCount:  job.fetchcount,
		MemLimit:    job.memlimit,
		CpuLimit:    job.cpulimit,
		Timeout:     job.timeout,
		SubmitTime:  (job.submittime).Format("2006-01-02 15:04:05"),
		StartTime:   start,
		EndTime:     end,
	}

	job.lock.RUnlock()

	if job.islong {
		if job.stream {
			// long job with streaming capability.
			info.OutputLink = fmt.Sprintf("https://%s:%s/worker/web/v1/jobs/long/output/stream?id=%s", Config.HttpsServerHost, Config.HttpsServerPort, job.id)
		} else if job.dump {
			// for long job with dump to file only option, we should download the file.
			info.OutputLink = fmt.Sprintf("https://%s:%s/worker/web/v1/jobs/x/output/download?id=%s", Config.HttpsServerHost, Config.HttpsServerPort, job.id)
		} else {
			// long job with no web & file stream options -> /dev/null.
			info.OutputLink = "n/a"
		}
	} else {
		// short job.
		info.OutputLink = fmt.Sprintf("https://%s:%s/worker/api/v1/jobs/fetch?id=%s", Config.HttpsServerHost, Config.HttpsServerPort, job.id)
	}

	return info
}

// setFailedInfos sets basics details for a completed or failed to start job.
func (job *Job) setFailedInfos(t time.Time, e string) {
	job.lock.Lock()
	job.endtime = t
	job.iscompleted = true
	job.issuccess = false
	job.errormsg = e
	job.lock.Unlock()
}

// formatJobsStatusTableHeaders constructs and returns the headers of the jobs status summary table.
func formatJobsStatusTableHeaders() string {
	var sb strings.Builder
	title := fmt.Sprintf("|%-4s | %-16s | %-5s | %-5s | %-5s | %-7s | %-9s | %-9s | %-5s | %-5s | %-6s | %-7s | %-20s | %-20s | %-20s | %-30s |", "Nb", "Job ID", "PID", "Long", "Done", "Success", "Exit Code", "Data [KB]", "Count", "Mem", "CPU%%", "Timeout", "Submitted At [UTC]", "Started At [UTC]", "Ended At [UTC]", "Command Syntax")
	// minus 1 since CPU percentage symbol was escaped (it counts 1 word in reality).
	sb.WriteString(strings.Repeat("=", len(title)-1))
	sb.WriteByte('\n')
	sb.WriteString(title)
	sb.WriteByte('\n')
	fmt.Fprintf(&sb, "+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(16), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(9), Dashs(9), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(20), Dashs(20), Dashs(20), Dashs(30))
	return sb.String()
}

// formatStatusAsTableRow constructs and returns a given row content for the jobs status summary table.
func (job *Job) formatStatusAsTableRow(i int, start, end, sizeFormat string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "|%04d | %-16s | %05d | %-5v | %-5v | %-7v | %-9d | %-9s | %-5d | %-5d | %-5d | %-7d | %-20v | %-20v | %-20v | %-30s |", i, job.id, job.pid, job.islong, job.iscompleted, job.issuccess, job.exitcode, sizeFormat, job.fetchcount, job.memlimit, job.cpulimit, job.timeout, (job.submittime).Format("2006-01-02 15:04:05"), start, end, truncateSyntax(job.task, 30))
	sb.WriteByte('\n')
	fmt.Fprintf(&sb, "+%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+-%s-+\n", Dashs(4), Dashs(16), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(9), Dashs(9), Dashs(5), Dashs(5), Dashs(5), Dashs(7), Dashs(20), Dashs(20), Dashs(20), Dashs(30))
	return sb.String()
}
