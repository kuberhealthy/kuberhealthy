package v1

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KuberhealthyCheck represents the data in the CRD for configuring an
// external check for Kuberhealthy
// +k8s:openapi-gen=true
// +kubebuilder:resource:path="khchecks"
// +kubebuilder:resource:singular="khcheck"
// +kubebuilder:resource:shortName="khc"
type KuberhealthyCheck struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the KuberhealthyCheck (from the client).
	// +optional
	Spec CheckConfig `json:"spec,omitempty"`
}

// CheckConfig represents a configuration for a kuberhealthy external
// check. This includes the pod spec to run, and the whitelisted
// UUID that is currently allowed to report-in to the status reporting
// endpoint.
// +k8s:openapi-gen=true
type CheckConfig struct {
	RunInterval      string            `json:"runInterval"`      // the interval at which the check runs
	Timeout          string            `json:"timeout"`          // the maximum time the pod is allowed to run before a failure is assumed
	PodSpec          apiv1.PodSpec     `json:"podSpec"`          // a spec for the external checker
	// +optional
	ExtraAnnotations map[string]string `json:"extraAnnotations"` // a map of extra annotations that will be applied to the pod
	// +optional
	ExtraLabels      map[string]string `json:"extraLabels"`      // a map of extra labels that will be applied to the pod
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KuberhealthyCheckList is a list of KuberhealthyCheck resources
type KuberhealthyCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []KuberhealthyCheck `json:"items"`
}
