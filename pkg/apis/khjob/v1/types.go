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
type KuberhealthyJob struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the Star (from the client).
	// +optional
	Spec JobConfig `json:"spec,omitempty"`
}

// JobConfig represents a configuration for a kuberhealthy external
// checker job. This includes the pod spec to run, and the whitelisted
// UUID that is currently allowed to report-in to the status reporting
// endpoint.
// +k8s:openapi-gen=true
type JobConfig struct {
	Timeout          string            `json:"timeout"`          // the maximum time the pod is allowed to run before a failure is assumed
	PodSpec          apiv1.PodSpec     `json:"podSpec"`          // a spec for the external checker
	ExtraAnnotations map[string]string `json:"extraAnnotations"` // a map of extra annotations that will be applied to the pod
	ExtraLabels      map[string]string `json:"extraLabels"`      // a map of extra labels that will be applied to the pod
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StarList is a list of Star resources
type KuberhealthyJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []KuberhealthyJob `json:"items"`
}
