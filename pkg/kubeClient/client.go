package kubeclient

import (
	"fmt"

	kuberhealthycheckv2 "github.com/kuberhealthy/crds/api/v2"
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

// NewKHClient creates a new kubernetes client with kuberhealthy CRDs working.  This is useful, but
// it's probably better to use the KHClient struct that has pre-made CRUD operations on it. To get
// a new KHClient struct, use New().
func NewKHClient() (client.Client, error) {
	// Create a new kubernetes client Scheme
	khScheme := runtime.NewScheme()
	err := clientgoscheme.AddToScheme(khScheme)
	if err != nil {
		return nil, fmt.Errorf("failed to add custom scheme: %v", err)
	}

	// Add Kubernetes core types to the scheme
	if err := scheme.AddToScheme(khScheme); err != nil {
		return nil, fmt.Errorf("failed to add Kubernetes core scheme to client: %v", err)
	}

	// Add Kuberhealthy's custom CRDs to the scheme
	if err = kuberhealthycheckv2.AddToScheme(khScheme); err != nil {
		return nil, fmt.Errorf("failed to add Kuberhealthy's custom CRDs to scheme: %v", err)
	}

	// Create an in cluster REST config for use with a new client
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %v", err)
	}

	// Create a dynamic client with the customScheme and in cluster config
	// then set the client globally
	kubeClient, err := client.New(config, client.Options{Scheme: khScheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create kuberhealthy-enabled kubernetes client: %v", err)
	}

	return kubeClient, nil
}
