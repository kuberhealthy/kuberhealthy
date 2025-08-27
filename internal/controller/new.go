package controller

import (
	"context"
	"fmt"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/kuberhealthy"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New creates a new KuberhealthyCheckReconciler with a working controller manager from the kubebuilder packages.
// Expects a kuberhealthy.Kuberhealthy. If it is not started, then this function will start it.
func New(ctx context.Context, cfg *rest.Config) (*KHCheckController, error) {
	log.Debugln("controller: starting new Kuberhealthy Controller")

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(khcrdsv2.AddToScheme(scheme))

	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("controller: error creating client: %w", err)
	}

	// make a new Kuberhealthy instance
	kh := kuberhealthy.New(ctx, cl)

	// Create the KHCheck controller
	khController, err := newKHCheckController(cfg, cl, scheme)
	if err != nil {
		return nil, fmt.Errorf("controller: error creating custom controller: %w", err)
	}
	khController.Kuberhealthy = kh

	// Start Kuberhealthy if needed
	if !kh.IsStarted() {
		if err := kh.Start(ctx); err != nil {
			return nil, fmt.Errorf("controller: error starting kuberhealthy: %w", err)
		}
	}

	// Start the controller
	go khController.Start(ctx)

	return khController, nil
}
