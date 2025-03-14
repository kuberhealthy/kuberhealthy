package kubeclient

import (
	"context"
	"fmt"

	kuberhealthycheckv2 "github.com/kuberhealthy/crds/api/v2"
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

// GetKuberhealthyCheck fetches a KuberhealthyCheck resource.
func (khc *KHClient) GetKuberhealthyCheck(name, namespace string) (*kuberhealthycheckv2.KuberhealthyCheck, error) {
	ctx := context.Background()
	key := client.ObjectKey{Name: name, Namespace: namespace}
	khCheck := &kuberhealthycheckv2.KuberhealthyCheck{}

	if err := khc.Client.Get(ctx, key, khCheck); err != nil {
		return nil, fmt.Errorf("error fetching KuberhealthyCheck: %w", err)
	}
	return khCheck, nil
}

// CreateKuberhealthyCheck creates a new KuberhealthyCheck resource.
func (khc *KHClient) CreateKuberhealthyCheck(khCheck *kuberhealthycheckv2.KuberhealthyCheck) error {
	ctx := context.Background()
	return khc.Client.Create(ctx, khCheck)
}

// UpdateKuberhealthyCheck updates an existing KuberhealthyCheck resource.
func (khc *KHClient) UpdateKuberhealthyCheck(khCheck *kuberhealthycheckv2.KuberhealthyCheck) error {
	ctx := context.Background()
	return khc.Client.Update(ctx, khCheck)
}

// ListKuberhealthyChecks lists all KuberhealthyCheck resources in a given namespace with optional ListOptions.
func (khc *KHClient) ListKuberhealthyChecks(namespace string, opts *client.ListOptions) (*kuberhealthycheckv2.KuberhealthyCheckList, error) {
	ctx := context.Background()
	khCheckList := &kuberhealthycheckv2.KuberhealthyCheckList{}
	opts.Namespace = namespace // enforce the namespace setting that the user supplied

	err := khc.Client.List(ctx, khCheckList, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing KuberhealthyChecks: %w", err)
	}
	return khCheckList, nil
}

// DeleteKuberhealthyCheck deletes a KuberhealthyCheck resource with optional DeleteOptions.
func (khc *KHClient) DeleteKuberhealthyCheck(name, namespace string, opts *metav1.DeleteOptions) error {
	ctx := context.Background()
	khCheck, err := khc.GetKuberhealthyCheck(name, namespace)
	if err != nil {
		return err
	}

	deleteOpts := []client.DeleteOption{}
	if opts != nil {
		deleteOpts = append(deleteOpts, client.PropagationPolicy(*opts.PropagationPolicy))
	}

	return khc.Client.Delete(ctx, khCheck, deleteOpts...)
}
