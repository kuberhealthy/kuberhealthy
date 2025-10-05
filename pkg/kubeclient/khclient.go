package kubeclient

import (
	"context"
	"fmt"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KHClient contains a featureful abstraction of the basic Kubernetes
// machinery, schemes, and clients that enables easy access to Kuberhealthy
// custom resources along with all basic Kubernetes client resource types.
type KHClient struct {
	Client client.Client
}

// New creates a new KHClient struct with working Kubernetes client
func New() (*KHClient, error) {
	crdClient, err := NewClient()
	if err != nil {
		return nil, err
	}

	return &KHClient{Client: crdClient}, nil
}

// GetHealthCheck fetches a HealthCheck resource.
func (khc *KHClient) GetHealthCheck(name, namespace string) (*khapi.HealthCheck, error) {
	ctx := context.Background()
	nn := client.ObjectKey{Name: name, Namespace: namespace}

	khCheck, err := khapi.GetCheck(ctx, khc.Client, nn)
	if err != nil {
		return nil, fmt.Errorf("error fetching HealthCheck: %w", err)
	}
	return khCheck, nil
}

// CreateHealthCheck creates a new HealthCheck resource.
func (khc *KHClient) CreateHealthCheck(khCheck *khapi.HealthCheck) error {
	ctx := context.Background()
	return khc.Client.Create(ctx, khCheck)
}

// UpdateHealthCheck updates an existing HealthCheck resource.
func (khc *KHClient) UpdateHealthCheck(khCheck *khapi.HealthCheck) error {
	ctx := context.Background()
	return khc.Client.Update(ctx, khCheck)
}

// ListHealthChecks lists all HealthCheck resources in a given namespace with optional ListOptions.
func (khc *KHClient) ListHealthChecks(namespace string, opts *client.ListOptions) (*khapi.HealthCheckList, error) {
	ctx := context.Background()
	khCheckList := &khapi.HealthCheckList{}
	opts.Namespace = namespace // enforce the namespace setting that the user supplied

	err := khc.Client.List(ctx, khCheckList, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing HealthChecks: %w", err)
	}
	for i := range khCheckList.Items {
		khCheckList.Items[i].EnsureCreationTimestamp()
	}
	return khCheckList, nil
}

// DeleteHealthCheck deletes a HealthCheck resource with optional DeleteOptions.
func (khc *KHClient) DeleteHealthCheck(name, namespace string, opts *metav1.DeleteOptions) error {
	ctx := context.Background()
	khCheck, err := khc.GetHealthCheck(name, namespace)
	if err != nil {
		return err
	}

	deleteOpts := []client.DeleteOption{}
	if opts != nil {
		deleteOpts = append(deleteOpts, client.PropagationPolicy(*opts.PropagationPolicy))
	}

	return khc.Client.Delete(ctx, khCheck, deleteOpts...)
}
