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

package api

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// CheckPodMetadata holds pod metadata limited to labels and annotations.
type CheckPodMetadata struct {
	// Labels applied to the checker pod.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations applied to the checker pod.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// CheckPodSpec contains the pod spec and optional metadata for a check.
type CheckPodSpec struct {
	// Metadata contains labels and annotations for the pod.
	Metadata *CheckPodMetadata `json:"metadata,omitempty"`
	// Spec is the full PodSpec for the checker pod.
	Spec corev1.PodSpec `json:"spec,omitempty"`
}

// HealthCheckSpec defines the desired state of a HealthCheck.
type HealthCheckSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// SingleRun indicates that this HealthCheck will run only once.
	SingleRun bool `json:"singleRunOnly,omitempty"`
	// RunInterval specifies how often Kuberhealthy schedules the check.
	RunInterval *metav1.Duration `json:"runInterval,omitempty"`
	// Timeout defines how long Kuberhealthy waits for the check to finish.
	Timeout *metav1.Duration `json:"timeout,omitempty"`
	// ExtraAnnotations are added to all checker pods.
	ExtraAnnotations map[string]string `json:"extraAnnotations,omitempty"`
	// ExtraLabels are applied to all checker pods.
	ExtraLabels map[string]string `json:"extraLabels,omitempty"`
	// PodSpec defines the pod executed for this check.
	// omitzero keeps an unset pod spec out of updates for HTTP and TCP checks.
	PodSpec CheckPodSpec `json:"podSpec,omitzero"`
	// HTTP makes Kuberhealthy send this HTTP request itself instead of running a checker pod.
	HTTP *HTTPCheck `json:"http,omitempty"`
	// TCP makes Kuberhealthy open this TCP connection itself instead of running a checker pod.
	TCP *TCPCheck `json:"tcp,omitempty"`
}

// HTTPCheck describes an HTTP request that Kuberhealthy sends from its own pod.
type HTTPCheck struct {
	// URL is the http or https address to request.
	URL string `json:"url"`
	// Method is the HTTP method to use. Defaults to GET.
	Method string `json:"method,omitempty"`
	// ExpectedStatusCodes lists the status codes that count as success. Defaults to any 2xx code.
	ExpectedStatusCodes []int `json:"expectedStatusCodes,omitempty"`
	// BodyContains is a string that the response body must contain.
	BodyContains string `json:"bodyContains,omitempty"`
	// BodyRegex is a regular expression that the response body must match.
	BodyRegex string `json:"bodyRegex,omitempty"`
	// Headers are added to the request.
	Headers map[string]string `json:"headers,omitempty"`
	// InsecureSkipVerify disables TLS certificate verification.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
	// MinCertificateValidity fails the check when a certificate served for an https URL expires sooner than this.
	MinCertificateValidity *metav1.Duration `json:"minCertificateValidity,omitempty"`
}

// TCPCheck describes a TCP connection that Kuberhealthy opens from its own pod.
type TCPCheck struct {
	// Address is the host:port to connect to.
	Address string `json:"address"`
}

// IsBuiltIn reports whether the check is run by Kuberhealthy itself rather than by a checker pod.
func (s *HealthCheckSpec) IsBuiltIn() bool {
	// an HTTP probe is run in the controller
	if s.HTTP != nil {
		return true
	}

	// a TCP probe is run in the controller too
	return s.TCP != nil
}

// HealthCheckStatus defines the observed state of a HealthCheck.
type HealthCheckStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// OK indicates if this check is currently throwing an error or not.
	OK bool `json:"ok"`
	// Errors holds a slice of error messages from the check results.
	Errors []string `json:"errors,omitempty"`
	// ConsecutiveFailures tracks the number of sequential failed runs.
	ConsecutiveFailures int `json:"consecutiveFailures,omitempty"`
	// LastRunDuration is the execution time that the checker pod took to execute.
	LastRunDuration time.Duration `json:"runDuration,omitempty"`
	// Namespace is the Kubernetes namespace this pod ran in.
	Namespace string `json:"namespace,omitempty"`
	// CurrentUUID is used to ensure only the most recent checker pod reports a status for this check.
	// Do not omit this field when empty so that a status update can explicitly
	// clear the UUID after a run completes, allowing the scheduler to start
	// subsequent runs.
	CurrentUUID string `json:"currentUUID"`
	// LastRunUnix is the last time that this check was scheduled to run.
	LastRunUnix int64 `json:"lastRunUnix,omitempty"`
	// SuccessCount tracks the total number of successful runs.
	SuccessCount int `json:"successCount,omitempty"`
	// FailureCount tracks the total number of failed runs.
	FailureCount int `json:"failureCount,omitempty"`
	// LastOKUnix is the last time this check reported OK.
	LastOKUnix int64 `json:"lastOKUnix,omitempty"`
	// LastFailureUnix is the last time this check reported a failure.
	LastFailureUnix int64 `json:"lastFailureUnix,omitempty"`
	// AdditionalMetadata is used to store additional metadata bout this check that appears in the JSON status.
	AdditionalMetadata string `json:"additionalMetadata,omitempty"`
	// Additional derived timing fields like next run and current runtime are
	// calculated from LastRunUnix by clients and are not stored on the
	// custom resource.
}

// HealthCheck is the Schema for the healthcheck API.
type HealthCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HealthCheckSpec   `json:"spec,omitempty"`
	Status HealthCheckStatus `json:"status,omitempty"`
}

// HealthCheckList contains a list of HealthCheck.
type HealthCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HealthCheck `json:"items"`
}

// init registers the custom resource types with the shared scheme builder.
func init() {
	SchemeBuilder.Register(&HealthCheck{}, &HealthCheckList{})
}

// CurrentUUID returns the running UUID for this check.
func (k *HealthCheck) CurrentUUID() string {
	return k.Status.CurrentUUID
}

// SetCurrentUUID updates the running UUID on the check status.
func (k *HealthCheck) SetCurrentUUID(u string) {
	k.Status.CurrentUUID = u
}

// SetOK marks the check status as healthy.
func (k *HealthCheck) SetOK() {
	k.Status.OK = true
}

// SetNotOK marks the check status as unhealthy.
func (k *HealthCheck) SetNotOK() {
	k.Status.OK = false
}

// SetCheckExecutionError assigns execution errors on the check status.
func (k *HealthCheck) SetCheckExecutionError(errs []string) {
	k.Status.Errors = errs
}

// EnsureCreationTimestamp sets CreationTimestamp to now when unset.
// It returns true when the timestamp was modified.
func (k *HealthCheck) EnsureCreationTimestamp() bool {
	if k.CreationTimestamp.IsZero() {
		k.CreationTimestamp = metav1.NewTime(time.Now())
		return true
	}
	return false
}

// CreateCheck writes a new check object to the cluster.
func CreateCheck(ctx context.Context, cl client.Client, check *HealthCheck) error {
	return cl.Create(ctx, check)
}

// GetCheck fetches the current version of a check.
func GetCheck(ctx context.Context, cl client.Client, nn types.NamespacedName) (*HealthCheck, error) {
	out := &HealthCheck{}
	err := cl.Get(ctx, nn, out)
	if err != nil {
		return nil, err
	}
	if out.EnsureCreationTimestamp() {
		err = cl.Update(ctx, out)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// UpdateCheck persists status changes for a check.
func UpdateCheck(ctx context.Context, cl client.Client, check *HealthCheck) error {
	return cl.Status().Update(ctx, check)
}

// DeleteCheck removes a check from the cluster.
func DeleteCheck(ctx context.Context, cl client.Client, check *HealthCheck) error {
	return cl.Delete(ctx, check)
}
