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
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the KuberhealthyState (from the client).
	// +optional
	Spec WorkloadDetails `json:"spec"`
}

// WorkloadDetails contains details about a single kuberhealthy check or job's current status
// +k8s:openapi-gen=true
type WorkloadDetails struct {
	OK               bool      `json:"OK"`               // true or false status of the khWorkload, whether or not it completed successfully
	Errors           []string  `json:"Errors"`           // the list of errors reported from the khWorkload run
	RunDuration      string    `json:"RunDuration"`      // the time it took for the khWorkload to complete
	Namespace        string    `json:"Namespace"`        // the namespace the khWorkload was run in
	Node 			 string    `json:"Node"`		     // the node the khWorkload ran on
	// +optional
	LastRun          metav1.Time `json:"LastRun"`          // the time the khWorkload was last run
	AuthoritativePod string    `json:"AuthoritativePod"` // the main kuberhealthy pod creating and updating the khstate
	CurrentUUID      string    `json:"uuid"`             // the UUID that is authorized to report statuses into the kuberhealthy endpoint
	khWorkload       KHWorkload
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
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []KuberhealthyState `json:"items"`
}
