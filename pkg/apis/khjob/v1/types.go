package v1

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KuberhealthyJob represents the data in the CRD for configuring an
// external checker job for Kuberhealthy
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="OK",type=string,JSONPath=`.status.ok`,description="OK status"
// +kubebuilder:printcolumn:name="Age LastRun",type=date,JSONPath=`.status.lastRun`,description="Last Run"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="Age"
// +kubebuilder:resource:path="khjobs"
// +kubebuilder:resource:singular="khjob"
// +kubebuilder:resource:shortName="khj"
// +kubebuilder:subresource:status
type KuberhealthyJob struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec holds the desired state of the KuberhealthyJob (from the client).
	// +optional
	Spec JobConfig `json:"spec,omitempty" yaml:"spec,omitempty"`

	// Status holds the results of the job
	Status JobStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// JobConfig represents a configuration for a kuberhealthy external
// checker job. This includes the pod spec to run, and the whitelisted
// UUID that is currently allowed to report-in to the status reporting
// endpoint.
// +k8s:openapi-gen=true
type JobConfig struct {
	// +optional
	Phase   JobPhase      `json:"phase" yaml:"phase"`     // the state or phase of the job
	Timeout string        `json:"timeout" yaml:"timeout"` // the maximum time the pod is allowed to run before a failure is assumed
	PodSpec apiv1.PodSpec `json:"podSpec" yaml:"podSpec"` // a spec for the external job
	// +optional
	ExtraAnnotations map[string]string `json:"extraAnnotations" yaml:"extraAnnotations"` // a map of extra annotations that will be applied to the pod
	// +optional
	ExtraLabels map[string]string `json:"extraLabels" yaml:"extraLabels"` // a map of extra labels that will be applied to the pod
}

type JobStatus struct {
	OK          bool     `json:"ok" yaml:"ok"`                   // true or false status of the job, whether or not it completed successfully
	Errors      []string `json:"errors" yaml:"errors"`           // the list of errors reported from the job run
	RunDuration string   `json:"runDuration" yaml:"runDuration"` // the time it took for the job to complete
	Node        string   `json:"node" yaml:"node"`               // the node the job ran on
	// +nullable
	LastRun          *metav1.Time `json:"lastRun,omitempty" yaml:"lastRun,omitempty"` // the time the job was last run
	AuthoritativePod string       `json:"authoritativePod" yaml:"authoritativePod"`   // the main kuberhealthy pod creating and updating the state
	CurrentUUID      string       `json:"uuid" yaml:"uuid"`                           // the UUID that is authorized to report statuses into the kuberhealthy endpoint
}

// JobPhase is a label for the condition of the job at the current time.
type JobPhase string

// These are the valid phases of jobs.
const (
	JobRunning   JobPhase = "Running"
	JobCompleted JobPhase = "Completed"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KuberhealthyJobList is a list of KuberhealthyJob resources
type KuberhealthyJobList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`

	Items []KuberhealthyJob `json:"items" yaml:"items"`
}
