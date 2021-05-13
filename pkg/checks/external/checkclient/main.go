// Package checkclient implements a client for reporting the state of an
// externally spawned checker pod to Kuberhealthy.  The URL that reports are
// sent to are pulled from the environment variables of the pod because
// Kuberhealthy sets them all all external checkers when they are spawned.
package checkclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cenkalti/backoff"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/status"
)

var (
	// Debug can be used to enable output logging from the checkClient
	Debug bool
)

// Use exponential backoff for retries
const maxElapsedTime = time.Second * 30

// ReportSuccess reports a successful check run to the Kuberhealthy service. We
// do not return an error here because failures will cause the managing
// instance of Kuberhealthy to time out and show an error.
func ReportSuccess() error {
	writeLog("DEBUG: Reporting SUCCESS")

	// make a new report without errors
	newReport := status.NewReport([]string{})

	// send the payload
	return sendReport(newReport)
}

// ReportFailure reports that the external checker has found problems.  You may
// pass a slice of error message strings that will surface in the Kuberhealthy
// status page for more context on the failure.  We do not return an error here
// because the managing instance of Kuberhealthy for this check will detect the
// failure to report-in and raise an error upstream.
func ReportFailure(errorMessages []string) error {
	writeLog("DEBUG: Reporting FAILURE")

	// make a new report without errors
	newReport := status.NewReport(errorMessages)

	// send it
	return sendReport(newReport)
}

// writeLog writes a log entry if debugging is enabled
func writeLog(i ...interface{}) {
	if Debug {
		log.Println("checkClient:", fmt.Sprint(i...))
	}
}

// sendReport marshals the report and sends it to the kuberhealthy endpoint
// as shown in the environment variables.
func sendReport(s status.Report) error {

	writeLog("DEBUG: Sending report with error length of:", len(s.Errors))
	writeLog("DEBUG: Sending report with ok state of:", s.OK)

	// marshal the request body
	b, err := json.Marshal(s)
	if err != nil {
		writeLog("ERROR: Failed to marshal status JSON:", err)
		return fmt.Errorf("error mashaling status report json: %w", err)
	}

	// fetch the server url
	url, err := getKuberhealthyURL()
	if err != nil {
		return fmt.Errorf("failed to fetch the kuberhealthy url: %w", err)
	}
	writeLog("INFO: Using kuberhealthy reporting URL: ", url)

	// fetch the kh run UUID
	uuid, err := getKuberhealthyRunUUID()
	if err != nil {
		return fmt.Errorf("failed to fetch the kuberhealthy run uuid: %w", err)
	}
	writeLog("INFO: Using kuberhealthy run UUID: ", uuid)

	// create the Kuberhealthy post request with the kh-run-uuid header
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("error creating http request: %w", err)
	}
	req.Header.Set("kh-run-uuid", uuid)
	req.Header.Set("Content-Type", "application/json")

	exponentialBackOff := backoff.NewExponentialBackOff()
	exponentialBackOff.MaxElapsedTime = maxElapsedTime

	client := &http.Client{}
	// send to the server
	var resp *http.Response
	err = backoff.Retry(func() error {
		var err error
		writeLog("DEBUG: Making POST request to kuberhealthy:")
		resp, err = client.Do(req)
		// retry on any errors
		if err != nil {
			return err
		}
		// retry on status codes that do not return a 200 or 400
		if !(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest) {
			writeLog("ERROR: got a bad status code from kuberhealthy:", resp.StatusCode, resp.Status)
			return fmt.Errorf("bad status code from kuberhealthy status reporting url: [%d] %s ", resp.StatusCode, resp.Status)
		}
		return nil
	}, exponentialBackOff)
	if err != nil {
		writeLog("ERROR: got an error sending POST to kuberhealthy:", err)
		return fmt.Errorf("bad POST request to kuberhealthy status reporting url: %w", err)
	}

	writeLog("INFO: Got a good http return status code from kuberhealthy URL:", url)

	return err
}

// getKuberhealthyURL fetches the URL that we need to send our external checker
// status report to from the environment variables
func getKuberhealthyURL() (string, error) {

	reportingURL := os.Getenv(external.KHReportingURL)

	// check the length of the reporting url to make sure we pulled one properly
	if len(reportingURL) < 1 {
		writeLog("ERROR: kuberhealthy reporting URL from environment variable", external.KHReportingURL, "was blank")
		return "", fmt.Errorf("fetched %s environment variable but it was blank", external.KHReportingURL)
	}

	return reportingURL, nil
}

// getKuberhealthyRunUUID fetches the kuberheathy checker pod run UUID to send to our external checker
// status to report to from the environment variable
func getKuberhealthyRunUUID() (string, error) {

	khRunUUID := os.Getenv(external.KHRunUUID)

	// check the length of the UUID to make sure we pulled one properly
	if len(khRunUUID) < 1 {
		writeLog("ERROR: kuberhealthy run UUID from environment variable", external.KHRunUUID, "was blank")
		return "", fmt.Errorf("fetched %s environment variable but it was blank", external.KHRunUUID)
	}

	return khRunUUID, nil
}

// GetDeadline fetches the KH_CHECK_RUN_DEADLINE environment variable and returns it.
// Checks are given up to the deadline to complete their check runs.
func GetDeadline() (time.Time, error) {
	unixDeadline := os.Getenv(external.KHDeadline)

	if len(unixDeadline) < 1 {
		writeLog("ERROR: kuberhealthy check deadline from environment variable", external.KHDeadline, "was blank")
		return time.Time{}, fmt.Errorf("fetched %s environment variable but it was blank", external.KHDeadline)
	}

	unixDeadlineInt, err := strconv.Atoi(unixDeadline)
	if err != nil {
		writeLog("ERROR: unable to parse", external.KHDeadline+": "+err.Error())
		return time.Time{}, fmt.Errorf("unable to parse %s: %s", external.KHDeadline, err.Error())
	}

	return time.Unix(int64(unixDeadlineInt), 0), nil
}
