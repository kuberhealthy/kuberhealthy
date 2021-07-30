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
// +kubebuilder:resource:path="khjobs"
// +kubebuilder:resource:singular="khjob"
// +kubebuilder:resource:shortName="khj"
type KuberhealthyJob struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec holds the desired state of the KuberhealthyJob (from the client).
	// +optional
	Spec JobConfig `json:"spec,omitempty" yaml:"spec,omitempty"`
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
