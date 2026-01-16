package health

import (
	"encoding/json"
	"net/http"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	log "github.com/sirupsen/logrus"
)

// State represents the results of all checks being managed along with a top-level OK and Error state. This is displayed
// on the kuberhealthy status page as JSON
// CheckDetail represents the status of a single check along with the
// next time the check is expected to run.  NextRunUnix is zero when the
// last run time is unknown.
type CheckDetail struct {
	khapi.HealthCheckStatus
	NextRunUnix        int64             `json:"nextRunUnix,omitempty"`
	PodName            string            `json:"podName,omitempty"`
	TimeoutSeconds     int64             `json:"timeoutSeconds,omitempty"`
	RunIntervalSeconds int64             `json:"runIntervalSeconds,omitempty"`
	Labels             map[string]string `json:"-"`
}

// State represents the results of all checks being managed along with a
// top-level OK and Error state. This is displayed on the kuberhealthy
// status page as JSON
type State struct {
	OK            bool
	Errors        []string
	CheckDetails  map[string]CheckDetail // map of job names to last run timestamp
	CurrentMaster string
	Metadata      map[string]string
	Controller    ControllerMetrics `json:"-"`
}

// ControllerMetrics captures controller-level statistics for metrics emission.
type ControllerMetrics struct {
	IsLeader                       bool
	SchedulerLoopDurationSeconds   float64
	SchedulerDueChecks             int
	ReaperLastSweepDurationSeconds float64
	ReaperDeletedPodsTotalByReason map[string]int64
}

// NewControllerMetrics builds a ControllerMetrics struct with initialized maps.
func NewControllerMetrics() ControllerMetrics {
	return ControllerMetrics{
		ReaperDeletedPodsTotalByReason: map[string]int64{},
	}
}

// AddError adds new errors to State
func (h *State) AddError(s ...string) {
	for _, str := range s {
		if len(str) == 0 {
			log.Warningln("AddError was called but the error was blank so it was skipped.")
			continue
		}
		log.Debugln("Appending error:", str)
		h.Errors = append(h.Errors, str)
	}
}

// WriteHTTPStatusResponse writes a response to an http response writer
func (h *State) WriteHTTPStatusResponse(w http.ResponseWriter) error {

	currentStatus := *h

	// marshal the health check results into a json blob of bytes
	b, err := json.MarshalIndent(currentStatus, "", "  ")
	if err != nil {
		log.Warningln("Error marshaling health check json for caller:", err)
		return err
	}

	// write the output to the caller
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = w.Write(b)
	if err != nil {
		log.Errorln("Error writing response to caller:", err)
		return err
	}

	return err
}

// NewState creates a new health check result response
func NewState() State {
	s := State{}
	s.OK = true
	s.Errors = []string{}
	s.CheckDetails = make(map[string]CheckDetail)
	s.Metadata = map[string]string{}
	s.Controller = NewControllerMetrics()
	return s
}
