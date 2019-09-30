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

	"github.com/Comcast/kuberhealthy/pkg/checks/external"
	"github.com/Comcast/kuberhealthy/pkg/checks/external/status"
)

// ReportSuccess reports a successful check run to the Kuberhealthy service. We
// do not return an error here because failures will cause the managing
// instance of Kuberhealthy to time out and show an error.
func ReportSuccess() error {

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

	// make a new report without errors
	newReport := status.NewReport(errorMessages)

	// send it
	return sendReport(newReport)
}

// sendReport marshals the report and sends it to the kuberhealthy endpoint
// as shown in the environment variables.
func sendReport(s status.Report) error {

	// marshal the request body
	b, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("error mashaling status report json: %w", err)
	}

	// fetch the server url
	url, err := getKuberhealthyURL()
	if err != nil {
		return fmt.Errorf("Failed to fetch the kuberhealthy url: %w", err)
	}

	// send to the server
	// TODO - retry logic?  Maybe we want this to be sensitive on a failure...
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))

	// make sure we got a 200 and consider it an error otherwise
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code from kuberhealthy status reporting url: %s", resp.Status)
	}

	return err
}

// getKuberhealthyURL fetches the URL that we need to send our external checker
// status report to from the environment variables
func getKuberhealthyURL() (string, error) {

	reportingURL := os.Getenv(external.KHReportingURL)

	// check the length of the reporting url to make sure we pulled one properly
	if len(reportingURL) < 1 {
		return "", fmt.Errorf("fetched %s environment variable but it was blank", external.KHReportingURL)
	}

	return reportingURL, nil
}
