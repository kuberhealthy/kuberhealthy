package main

import (
	"context"
	"log"

	comcastgithubiov1 "github.com/kuberhealthy/crds/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/scale/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// Import your custom API group/version/types
)

var (
	customScheme = runtime.NewScheme()
	setupLog     = ctrl.Log.WithName("setup")
	kubeClient   client.Client
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(customScheme))
	utilruntime.Must(comcastgithubiov1.AddToScheme(customScheme))
	// +kubebuilder:scaffold:scheme
}

// setupKubernetesClient creates a new kubernetes client with kuberhealthy CRDs working
func setupKubernetesClient() {
	// Create a new kubernetes client Scheme
	customScheme := runtime.NewScheme()

	// Add Kubernetes core types to the scheme
	if err := scheme.AddToScheme(customScheme); err != nil {
		log.Fatalf("failed to add Kubernetes core scheme: %v", err)
	}

	// Add Kuberhealthy's custom CRDs to the scheme
	if err := comcastgithubiov1.AddToScheme(customScheme); err != nil {
		log.Fatalf("failed to add custom scheme: %v", err)
	}

	// Create an in cluster REST config for use with a new client
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("failed to get Kubernetes config: %v", err)
	}

	// Create a dynamic client with the customScheme and in cluster config
	// then set the client globally
	clientOptions := client.Options{Scheme: customScheme}
	kubeClient, err = client.New(config, clientOptions)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	// set the client globally
	ctx := context.Background()
	key := client.ObjectKey{
		Namespace: "default",
		Name:      "example-configmap",
	}
	khState := comcastgithubiov1.KuberhealthyState{} // client will unmarshal into this target struct

	err = kubeClient.Get(ctx, key, &khState)
	if err != nil {
		// return fmt.Errorf("error fetching khstate: %w", err)
	}

}
