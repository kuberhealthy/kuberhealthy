package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KuberhealthyState represents the data in the CRD for configuring an
// the state of khjobs or khchecks for Kuberhealthy
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="OK",type=string,JSONPath=`.spec.OK`,description="OK status"
// +kubebuilder:printcolumn:name="Age LastRun",type=date,JSONPath=`.spec.LastRun`,description="Last Run"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="Age"
// +kubebuilder:resource:path="khstates"
// +kubebuilder:resource:singular="khstate"
// +kubebuilder:resource:shortName="khs"
type KuberhealthyState struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec holds the desired state of the KuberhealthyState (from the client).
	// +optional
	Spec WorkloadDetails `json:"spec" yaml:"spec"`
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
	// +nullable
	khWorkload *KHWorkload `json:"khWorkload,omitempty" yaml:"khWorkload,omitempty"`
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KuberhealthyStateList is a list of KuberhealthyState resources
type KuberhealthyStateList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`

	Items []KuberhealthyState `json:"items" yaml:"items"`
}
