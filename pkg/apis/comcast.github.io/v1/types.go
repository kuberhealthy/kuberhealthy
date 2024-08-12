package v1

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KuberhealthyCheck represents the data in the CRD for configuring an
// external check for Kuberhealthy
// +k8s:openapi-gen=true
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KuberhealthyCheck struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	// Spec holds the desired state of the KuberhealthyCheck (from the client).
	Spec CheckConfig `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// CheckConfig represents a configuration for a kuberhealthy external
// check. This includes the pod spec to run, and the whitelisted
// UUID that is currently allowed to report-in to the status reporting
// endpoint.
// +k8s:openapi-gen=true
type CheckConfig struct {
	RunInterval string        `json:"runInterval" yaml:"runInterval"` // the interval at which the check runs
	Timeout     string        `json:"timeout" yaml:"timeout"`         // the maximum time the pod is allowed to run before a failure is assumed
	PodSpec     apiv1.PodSpec `json:"podSpec" yaml:"podSpec"`         // a spec for the external checker
	// +optional
	ExtraAnnotations map[string]string `json:"extraAnnotations" yaml:"extraAnnotations"` // a map of extra annotations that will be applied to the pod
	// +optional
	ExtraLabels map[string]string `json:"extraLabels" yaml:"extraLabels"` // a map of extra labels that will be applied to the pod
}

// KuberhealthyCheckList is a list of KuberhealthyCheck resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KuberhealthyCheckList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`

	Items []KuberhealthyCheck `json:"items" yaml:"items"`
}

// KuberhealthyJob represents the data in the CRD for configuring an
// external checker job for Kuberhealthy
// +k8s:openapi-gen=true
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KuberhealthyJob struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	// Spec holds the desired state of the KuberhealthyJob (from the client).
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

// KuberhealthyJobList is a list of KuberhealthyJob resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KuberhealthyJobList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`

	Items []KuberhealthyJob `json:"items" yaml:"items"`
}

// WorkloadDetails contains details about a single kuberhealthy check or job's current status
// +k8s:openapi-gen=true
// +nullable:name="LastRun"
type WorkloadDetails struct {
	OK          bool     `json:"OK" yaml:"OK"`                   // true or false status of the khWorkload, whether or not it completed successfully
	Errors      []string `json:"Errors" yaml:"Errors"`           // the list of errors reported from the khWorkload run
	RunDuration string   `json:"RunDuration" yaml:"RunDuration"` // the time it took for the khWorkload to complete
	Namespace   string   `json:"Namespace" yaml:"Namespace"`     // the namespace the khWorkload was run in
	Node        string   `json:"Node" yaml:"Node"`               // the node the khWorkload ran on
	// +nullable
	LastRun          *metav1.Time `json:"LastRun,omitempty" yaml:"LastRun,omitempty"` // the time the khWorkload was last run
	AuthoritativePod string       `json:"AuthoritativePod" yaml:"AuthoritativePod"`   // the main kuberhealthy pod creating and updating the khstate
	CurrentUUID      string       `json:"uuid" yaml:"uuid"`                           // the UUID that is authorized to report statuses into the kuberhealthy endpoint
	KHWorkload       KHWorkload   `json:"khWorkload,omitempty" yaml:"khWorkload,omitempty"`
}

// KHWorkload is used to describe the different types of kuberhealthy workloads: KhCheck or KHJob
type KHWorkload string

// Two types of KHWorkloads are available: Kuberhealthy Check or Kuberhealthy Job
// KHChecks run on a scheduled run interval
// KHJobs run once
const (
	KHCheck KHWorkload = "KHCheck"
	KHJob   KHWorkload = "KHJob"
)

// KuberhealthyState represents the data in the CRD for configuring an
// the state of khjobs or khchecks for Kuberhealthy
// +k8s:openapi-gen=true
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KuberhealthyState struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec holds the desired state of the KuberhealthyState (from the client).
	// +optional
	Spec WorkloadDetails `json:"spec" yaml:"spec"`
}

// KuberhealthyStateList is a list of KuberhealthyState resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KuberhealthyStateList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`

	Items []KuberhealthyState `json:"items" yaml:"items"`
}
