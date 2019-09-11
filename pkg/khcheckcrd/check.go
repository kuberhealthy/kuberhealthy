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
	RunInterval string `json:"runInterval"`       // the interval at which the check runs
	PodSpec     apiv1.PodSpec `json:"podSpec"` // a spec for the external checker
	CurrentUUID string `json:"uuid"` // the UUID that is authorized to report statuses into the kuberhealthy endpoint
}

// NewCheckConfig creates a new check configuration
func NewCheckConfig(runInterval time.Duration, podSpec apiv1.PodSpec) CheckConfig {
	c := CheckConfig{
		RunInterval: runInterval.String(),
		PodSpec:     podSpec,
	}

	return c
}
