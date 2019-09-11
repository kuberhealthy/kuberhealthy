// Package status holds a struct that can be used to
// report external check status to the Kuberhealthy
// status reporting endpoint.
package status

// Report is the format expected by the /externalCheckStatus endpoint
type Report struct {
	UUID           string
	Errors         []string
	CheckName      string
	CheckNamespace string
}

// NewReport creates a new error report to be sent to the server.  If
// errors are left out, then we assume the status report is OK.  If
// any error is present, we assume the status is DOWN.
func NewReport(errorMessages []string) Report {
	return Report{
		Errors:         errorMessages,
	}
}
