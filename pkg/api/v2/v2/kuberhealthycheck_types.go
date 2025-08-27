/*
Copyright 2025 Kuberhealthy Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:validation:XPreserveUnknownFields
// KuberhealthyCheckSpec defines the desired state of KuberhealthyCheck
type KuberhealthyCheckSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// SingleRun indicates that this KuberhealthyCheck will run only once.
	SingleRun bool                   `json:"singleRunOnly,omitempty"`
	PodSpec   corev1.PodTemplateSpec `json:"podSpec,omitempty"`
	// PodSpec   v1.PodSpec              `json:"podSpec"` // We can not use a full PodSpec struct because it throws error about too much yaml in the CRD definition
}

// KuberhealthyCheckStatus defines the observed state of KuberhealthyCheck
type KuberhealthyCheckStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// OK indicates if this check is currently throwing an error or not.
	OK bool `json:"ok,omitempty"`
	// Errors holds a slice of error messages from the check results.
	Errors []string `json:"errors,omitempty"`
	// LastRunDuration is the execution time that the checker pod took to execute.
	LastRunDuration time.Duration `json:"runDuration,omitempty"`
	// Namespace is the Kubernetes namespace this pod ran in.
	Namespace string `json:"namespace,omitempty"`
	// PodName is the name of the Pod that was most recently created to run this check
	PodName string `json:"podName,omitempty"`
	// CurrentUUID is used to ensure only the most recent checker pod reports a status for this check.
	CurrentUUID string `json:"currentUUID,omitempty"`
	// LastRunUnix is the last time that this check was scheduled to run.
	LastRunUnix int64 `json:"lastRunUnix,omitempty"`
	// AdditionalMetadata is used to store additional metadata bout this check that appears in the JSON status.
	AdditionalMetadata string `json:"additionalMetadata,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=khc;khcheck;kuberhealthycheck

// KuberhealthyCheck is the Schema for the kuberhealthychecks API
type KuberhealthyCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KuberhealthyCheckSpec `json:"spec,omitempty"`
	// +optional
	Status KuberhealthyCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KuberhealthyCheckList contains a list of KuberhealthyCheck
type KuberhealthyCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KuberhealthyCheck `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KuberhealthyCheck{}, &KuberhealthyCheckList{})
}
