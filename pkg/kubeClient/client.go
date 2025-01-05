package kubeclient

import (
	"fmt"

	comcastgithubiov1 "github.com/kuberhealthy/crds/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/scale/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// Import your custom API group/version/types
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

// setupKubernetesClient creates a new kubernetes client with kuberhealthy CRDs working
func New() (client.Client, error) {
	// Create a new kubernetes client Scheme
	customScheme := runtime.NewScheme()
	err := clientgoscheme.AddToScheme(customScheme)
	if err != nil {
		return nil, fmt.Errorf("failed to add custom scheme: %v", err)
	}

	// Add Kubernetes core types to the scheme
	if err := scheme.AddToScheme(customScheme); err != nil {
		return nil, fmt.Errorf("failed to add Kubernetes core scheme to client: %v", err)
	}

	// Add Kuberhealthy's custom CRDs to the scheme
	if err = comcastgithubiov1.AddToScheme(customScheme); err != nil {
		return nil, fmt.Errorf("failed to add Kuberhealthy's custom CRDs to scheme: %v", err)
	}

	// Create an in cluster REST config for use with a new client
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %v", err)
	}

	// Create a dynamic client with the customScheme and in cluster config
	// then set the client globally
	kubeClient, err := client.New(config, client.Options{Scheme: customScheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create kuberhealthy-enabled kubernetes client: %v", err)
	}

	return kubeClient, nil
}
