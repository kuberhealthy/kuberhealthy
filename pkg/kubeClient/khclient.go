package kubeclient

import (
	"context"
	"fmt"

	comcastgithubiov1 "github.com/kuberhealthy/crds/api/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KHClient contains a featureful abstraction of the basic Kubernetes
// machinery, schemes, and clients that enables easy access to Kuberhealthy
// custom resources alog with all basic Kubernetes client resource types.
type KHClient struct {
	client.Client
}

// GetKuberhealthyState fetches a KuberhealthyState resoruce.
func (khc *KHClient) GetKuberhealthyState(name string, namespace string) (*comcastgithubiov1.KuberhealthyState, error) {
	// set the client globally
	ctx := context.Background()
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}
	khState := comcastgithubiov1.KuberhealthyState{} // client will unmarshal into this target struct

	err := khc.Get(ctx, key, &khState)
	if err != nil {
		return nil, fmt.Errorf("error fetching khstate: %w", err)
	}

	return &khState, nil

}
