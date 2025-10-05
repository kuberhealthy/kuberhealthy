// Package checkclient implements a client for reporting the state of an
// externally spawned checker pod to Kuberhealthy.  The URL that reports are
// sent to are pulled from the environment variables of the pod because
// Kuberhealthy sets them all all external checkers when they are spawned.
package checkclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cenkalti/backoff"
	log "github.com/sirupsen/logrus"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/envs"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
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
	newReport := khapi.HealthCheckStatus{}
	newReport.OK = true
	newReport.Errors = []string{}

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
	newReport := khapi.HealthCheckStatus{}
	newReport.OK = false
	newReport.Errors = errorMessages

	// send it
	return sendReport(newReport)
}

// writeLog writes a log entry if debugging is enabled
func writeLog(i ...interface{}) {
	if Debug {
		log.Infoln("checkClient:", fmt.Sprint(i...))
	}
}

// sendReport marshals the report and sends it to the kuberhealthy endpoint
// as shown in the environment variables.
func sendReport(s khapi.HealthCheckStatus) error {

	writeLog("DEBUG: Sending report with error length of:", len(s.Errors))
	writeLog("DEBUG: Sending report with ok state of:", s.OK)

	// marshal the request body so the API receives our status payload
	body, err := json.Marshal(s)
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

	// fetch the kh run UUID so the server can match the request to the pod
	uuid, err := getKuberhealthyRunUUID()
	if err != nil {
		return fmt.Errorf("failed to fetch the kuberhealthy run uuid: %w", err)
	}
	writeLog("INFO: Using kuberhealthy run UUID: ", uuid)

	// create the Kuberhealthy post request with the kh-run-uuid header
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("error creating http request: %w", err)
	}
	req.Header.Set("kh-run-uuid", uuid)
	req.Header.Set("Content-Type", "application/json")

	// configure a bounded exponential backoff so retries stop after a short window
	exponentialBackOff := backoff.NewExponentialBackOff()
	exponentialBackOff.MaxElapsedTime = maxElapsedTime

	// send to the server with a helper that implements the retry contract
	client := &http.Client{}
	sender := reportSender{
		client:  client,
		request: req,
	}
	err = backoff.Retry(sender.Attempt, exponentialBackOff)
	if err != nil {
		writeLog("ERROR: got an error sending POST to kuberhealthy:", err)
		return fmt.Errorf("bad POST request to kuberhealthy status reporting url: %w", err)
	}

	writeLog("INFO: Got a good http return status code from kuberhealthy URL:", url)

	return nil
}

// reportSender attempts to deliver a request to the kuberhealthy API with retries.
type reportSender struct {
	client  *http.Client
	request *http.Request
}

// Attempt sends the configured request and returns an error when a retry is needed.
func (r *reportSender) Attempt() error {
	// log the attempt so debugging shows each retry
	writeLog("DEBUG: Making POST request to kuberhealthy:")

	// send the request to the kuberhealthy server
	resp, err := r.client.Do(r.request)
	if err != nil {
		return err
	}
	// retry when kuberhealthy does not return a success or validation response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		writeLog("ERROR: got a bad status code from kuberhealthy:", resp.StatusCode, resp.Status)
		// close the response body because we will retry the request
		closeErr := resp.Body.Close()
		if closeErr != nil {
			return fmt.Errorf("failed to close response body after bad status: %w", closeErr)
		}
		return fmt.Errorf("bad status code from kuberhealthy status reporting url: [%d] %s ", resp.StatusCode, resp.Status)
	}

	// close successful responses because the caller does not inspect them
	closeErr := resp.Body.Close()
	if closeErr != nil {
		return fmt.Errorf("failed to close response body: %w", closeErr)
	}

	return nil
}

// getKuberhealthyURL fetches the URL that we need to send our external checker
// status report to from the environment variables
func getKuberhealthyURL() (string, error) {

	reportingURL := os.Getenv(envs.KHReportingURL)

	// check the length of the reporting url to make sure we pulled one properly
	if len(reportingURL) < 1 {
		writeLog("ERROR: kuberhealthy reporting URL from environment variable", envs.KHReportingURL, "was blank")
		return "", fmt.Errorf("fetched %s environment variable but it was blank", envs.KHReportingURL)
	}

	return reportingURL, nil
}

// getKuberhealthyRunUUID fetches the kuberheathy checker pod run UUID to send to our external checker
// status to report to from the environment variable
func getKuberhealthyRunUUID() (string, error) {

	khRunUUID := os.Getenv(envs.KHRunUUID)

	// check the length of the UUID to make sure we pulled one properly
	if len(khRunUUID) < 1 {
		writeLog("ERROR: kuberhealthy run UUID from environment variable", envs.KHRunUUID, "was blank")
		return "", fmt.Errorf("fetched %s environment variable but it was blank", envs.KHRunUUID)
	}

	return khRunUUID, nil
}

// GetDeadline fetches the KH_CHECK_RUN_DEADLINE environment variable and returns it.
// Checks are given up to the deadline to complete their check runs.
func GetDeadline() (time.Time, error) {
	unixDeadline := os.Getenv(envs.KHDeadline)

	if len(unixDeadline) < 1 {
		writeLog("ERROR: kuberhealthy check deadline from environment variable", envs.KHDeadline, "was blank")
		return time.Time{}, fmt.Errorf("fetched %s environment variable but it was blank", envs.KHDeadline)
	}

	unixDeadlineInt, err := strconv.Atoi(unixDeadline)
	if err != nil {
		writeLog("ERROR: unable to parse", envs.KHDeadline+": "+err.Error())
		return time.Time{}, fmt.Errorf("unable to parse %s: %s", envs.KHDeadline, err.Error())
	}

	return time.Unix(int64(unixDeadlineInt), 0), nil
}
