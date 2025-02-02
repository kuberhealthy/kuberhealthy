package health

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"

	comcastgithubiov1 "github.com/kuberhealthy/crds/api/v1"
)

// State represents the results of all checks being managed along with a top-level OK and Error state. This is displayed
// on the kuberhealthy status page as JSON
type State struct {
	OK            bool
	Errors        []string
	CheckDetails  map[string]comcastgithubiov1.KuberhealthyCheckSpec // map of check names to last run timestamp
	JobDetails    map[string]comcastgithubiov1.KuberhealthyJobSpec   // map of job names to last run timestamp
	CurrentMaster string
	Metadata      map[string]string
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
	s.CheckDetails = make(map[string]comcastgithubiov1.KuberhealthyCheckSpec)
	s.JobDetails = make(map[string]comcastgithubiov1.KuberhealthyJobSpec)
	s.Metadata = map[string]string{}
	return s
}
