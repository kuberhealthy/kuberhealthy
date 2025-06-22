package controller

import (
	"context"
	"fmt"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/kuberhealthy"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// New creates a new KuberhealthyCheckReconciler with a working controller manager from the kubebuilder packages.
// Expects a kuberhealthy.Kuberhealthy. If it is not started, then this function will start it.
func New(ctx context.Context) (*KuberhealthyCheckReconciler, error) {
	fmt.Println("Starting new Kuberhealthy Controller")

	scheme := runtime.NewScheme()
	utilruntime.Must(khcrdsv2.AddToScheme(scheme))

	// Get Kubernetes config
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("controller: error getting kubernetes config: %w", err)
	}

	// Create a new manager
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("controller: error creating manager: %w", err)
	}

	// make a new Kuberhealthy instance
	kh := kuberhealthy.New(ctx, mgr.GetClient())

	// Create and register the reconciler
	reconciler := &KuberhealthyCheckReconciler{
		Client:       mgr.GetClient(),
		Scheme:       scheme,
		Kuberhealthy: kh,
	}

	// Set the Kuberhealthy client and start it
	if !kh.IsStarted() {
		err := kh.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("controller: error starting kuberhealthy:", err)
		}
	}

	// Start the reconciler (controller)
	if err := reconciler.setupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("controller: error setting up controller with manager: %w", err)
	}

	// Start the manager with our reconciler in it
	err = mgr.Start(ctx)

	return reconciler, nil
}
