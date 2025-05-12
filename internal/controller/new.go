package controller

import (
	"context"
	"fmt"
	"log"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/kuberhealthy"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// New creates a new KuberhealthyCheckReconciler with a working controller manager from the kubebuilder packages.
// Expects a kuberhealthy.Kuberhealthy. If it is not started, then this function will start it.
func New(ctx context.Context, kh *kuberhealthy.Kuberhealthy) (*KuberhealthyCheckReconciler, error) {
	fmt.Println("-- controller New")

	// check if kuberhealthy is started
	if !kh.IsStarted() {
		err := kh.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("controller: error starting kuberhealthy:", err)
		}
	}

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

	// Create and register the reconciler
	reconciler := &KuberhealthyCheckReconciler{
		Client:       mgr.GetClient(),
		Scheme:       scheme,
		Kuberhealthy: kh,
	}

	if err := reconciler.setupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("controller: error setting up controller with manager: %w", err)
	}

	// Start the manager with our reconciler in it
	log.Println("-- controller start")
	err = mgr.Start(ctx)

	return reconciler, nil
}
