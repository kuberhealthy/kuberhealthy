package khcheckcrd

import (
	"time"

	apiv1 "k8s.io/api/core/v1"
)

// CheckConfig represents a configuration for a kuberhealthy external
// checker.  This includes the pod spec to run, the interval, and
// the whitelisted UUID that is currently allowed to report-in to
// the status reporting endpoint.
type CheckConfig struct {
	RunInterval      string            `json:"runInterval"`      // the interval at which the check runs
	Timeout          string            `json:"timeout"`          // the maximum time the pod is allowed to run before a failure is assumed
	PodSpec          apiv1.PodSpec     `json:"podSpec"`          // a spec for the external checker
	CurrentUUID      string            `json:"uuid"`             // the UUID that is authorized to report statuses into the kuberhealthy endpoint
	ExtraAnnotations map[string]string `json:"extraAnnotations"` // a map of extra annotations that will be applied to the pod
	ExtraLabels      map[string]string `json:"extraLabels"`      // a map of extra labels that will be applied to the pod
}

// DefaultTimeout is the default timeout for external checks
var DefaultTimeout = time.Minute * 5

// NewCheckConfig creates a new check configuration
func NewCheckConfig(runInterval time.Duration, podSpec apiv1.PodSpec) CheckConfig {

	c := CheckConfig{
		RunInterval:      runInterval.String(),
		Timeout:          DefaultTimeout.String(),
		ExtraAnnotations: make(map[string]string),
		ExtraLabels:      make(map[string]string),
		PodSpec:          podSpec,
	}

	return c
}
