package kubeclient

import (
	"context"
	"fmt"

	comcastgithubiov1 "github.com/kuberhealthy/crds/api/v1"
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
	crdClient, err := NewKHClient()
	if err != nil {
		return nil, err
	}

	return &KHClient{Client: crdClient}, nil
}

// GetKuberhealthyState fetches a KuberhealthyState resource.
func (khc *KHClient) GetKuberhealthyState(name, namespace string) (*comcastgithubiov1.KuberhealthyState, error) {
	ctx := context.Background()
	key := client.ObjectKey{Name: name, Namespace: namespace}
	khState := &comcastgithubiov1.KuberhealthyState{}

	if err := khc.Client.Get(ctx, key, khState); err != nil {
		return nil, fmt.Errorf("error fetching KuberhealthyState: %w", err)
	}
	return khState, nil
}

// CreateKuberhealthyState creates a new KuberhealthyState resource.
func (khc *KHClient) CreateKuberhealthyState(khState *comcastgithubiov1.KuberhealthyState) error {
	ctx := context.Background()
	return khc.Client.Create(ctx, khState)
}

// UpdateKuberhealthyState updates an existing KuberhealthyState resource.
func (khc *KHClient) UpdateKuberhealthyState(khState *comcastgithubiov1.KuberhealthyState) error {
	ctx := context.Background()
	return khc.Client.Update(ctx, khState)
}

// GetKuberhealthyCheck fetches a KuberhealthyCheck resource.
func (khc *KHClient) GetKuberhealthyCheck(name, namespace string) (*comcastgithubiov1.KuberhealthyCheck, error) {
	ctx := context.Background()
	key := client.ObjectKey{Name: name, Namespace: namespace}
	khCheck := &comcastgithubiov1.KuberhealthyCheck{}

	if err := khc.Client.Get(ctx, key, khCheck); err != nil {
		return nil, fmt.Errorf("error fetching KuberhealthyCheck: %w", err)
	}
	return khCheck, nil
}

// CreateKuberhealthyCheck creates a new KuberhealthyCheck resource.
func (khc *KHClient) CreateKuberhealthyCheck(khCheck *comcastgithubiov1.KuberhealthyCheck) error {
	ctx := context.Background()
	return khc.Client.Create(ctx, khCheck)
}

// UpdateKuberhealthyCheck updates an existing KuberhealthyCheck resource.
func (khc *KHClient) UpdateKuberhealthyCheck(khCheck *comcastgithubiov1.KuberhealthyCheck) error {
	ctx := context.Background()
	return khc.Client.Update(ctx, khCheck)
}

// GetKuberhealthyJob fetches a KuberhealthyJob resource.
func (khc *KHClient) GetKuberhealthyJob(name, namespace string) (*comcastgithubiov1.KuberhealthyJob, error) {
	ctx := context.Background()
	key := client.ObjectKey{Name: name, Namespace: namespace}
	khJob := &comcastgithubiov1.KuberhealthyJob{}

	if err := khc.Client.Get(ctx, key, khJob); err != nil {
		return nil, fmt.Errorf("error fetching KuberhealthyJob: %w", err)
	}
	return khJob, nil
}

// CreateKuberhealthyJob creates a new KuberhealthyJob resource.
func (khc *KHClient) CreateKuberhealthyJob(khJob *comcastgithubiov1.KuberhealthyJob) error {
	ctx := context.Background()
	return khc.Client.Create(ctx, khJob)
}

// UpdateKuberhealthyJob updates an existing KuberhealthyJob resource.
func (khc *KHClient) UpdateKuberhealthyJob(khJob *comcastgithubiov1.KuberhealthyJob) error {
	ctx := context.Background()
	return khc.Client.Update(ctx, khJob)
}

// ListKuberhealthyStates lists all KuberhealthyState resources in a given namespace with optional ListOptions.
func (khc *KHClient) ListKuberhealthyStates(namespace string, opts *metav1.ListOptions) (*comcastgithubiov1.KuberhealthyStateList, error) {
	ctx := context.Background()
	khStateList := &comcastgithubiov1.KuberhealthyStateList{}
	listOpts := []client.ListOption{client.InNamespace(namespace)}

	if opts != nil {
		if len(opts.LabelSelector) > 0 {
			listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: opts.LabelSelector})
		}
	}

	err := khc.Client.List(ctx, khStateList, listOpts...)
	if err != nil {
		return nil, fmt.Errorf("error listing KuberhealthyStates: %w", err)
	}
	return khStateList, nil
}

// ListKuberhealthyChecks lists all KuberhealthyCheck resources in a given namespace with optional ListOptions.
func (khc *KHClient) ListKuberhealthyChecks(namespace string, opts *metav1.ListOptions) (*comcastgithubiov1.KuberhealthyCheckList, error) {
	ctx := context.Background()
	khCheckList := &comcastgithubiov1.KuberhealthyCheckList{}
	listOpts := []client.ListOption{client.InNamespace(namespace)}

	if opts != nil {
		if len(opts.LabelSelector) > 0 {
			listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: opts.LabelSelector})
		}
	}

	err := khc.Client.List(ctx, khCheckList, listOpts...)
	if err != nil {
		return nil, fmt.Errorf("error listing KuberhealthyChecks: %w", err)
	}
	return khCheckList, nil
}

// ListKuberhealthyJobs lists all KuberhealthyJob resources in a given namespace with optional ListOptions.
func (khc *KHClient) ListKuberhealthyJobs(namespace string, opts *metav1.ListOptions) (*comcastgithubiov1.KuberhealthyJobList, error) {
	ctx := context.Background()
	khJobList := &comcastgithubiov1.KuberhealthyJobList{}
	listOpts := []client.ListOption{client.InNamespace(namespace)}

	if opts != nil {
		if len(opts.LabelSelector) > 0 {
			listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: opts.LabelSelector})
		}
	}

	err := khc.Client.List(ctx, khJobList, listOpts...)
	if err != nil {
		return nil, fmt.Errorf("error listing KuberhealthyJobs: %w", err)
	}
	return khJobList, nil
}

// DeleteKuberhealthyState deletes a KuberhealthyState resource with optional DeleteOptions.
func (khc *KHClient) DeleteKuberhealthyState(name, namespace string, opts *metav1.DeleteOptions) error {
	ctx := context.Background()
	khState, err := khc.GetKuberhealthyState(name, namespace)
	if err != nil {
		return err
	}

	deleteOpts := []client.DeleteOption{}
	if opts != nil {
		deleteOpts = append(deleteOpts, client.PropagationPolicy(*opts.PropagationPolicy))
	}

	return khc.Client.Delete(ctx, khState, deleteOpts...)
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

// DeleteKuberhealthyJob deletes a KuberhealthyJob resource with optional DeleteOptions.
func (khc *KHClient) DeleteKuberhealthyJob(name, namespace string, opts *metav1.DeleteOptions) error {
	ctx := context.Background()
	khJob, err := khc.GetKuberhealthyJob(name, namespace)
	if err != nil {
		return err
	}

	deleteOpts := []client.DeleteOption{}
	if opts != nil {
		deleteOpts = append(deleteOpts, client.PropagationPolicy(*opts.PropagationPolicy))
	}

	return khc.Client.Delete(ctx, khJob, deleteOpts...)
}
