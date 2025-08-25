package controller

import (
	"context"
	"fmt"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/kuberhealthy"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

// New creates a new KuberhealthyCheckReconciler with a working controller manager from the kubebuilder packages.
// Expects a kuberhealthy.Kuberhealthy. If it is not started, then this function will start it.
func New(ctx context.Context, cfg *rest.Config) (*KuberhealthyCheckReconciler, error) {
	log.Debugln("controller: starting new Kuberhealthy Controller")

	scheme := runtime.NewScheme()
	utilruntime.Must(khcrdsv2.AddToScheme(scheme))

	// Create a new manager with the default metrics server disabled.
	// Controller metrics will be served by the web server under /controllerMetrics.
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
			return nil, fmt.Errorf("controller: error starting kuberhealthy: %w", err)
		}
	}

	// Start the reconciler (controller)
	if err := reconciler.setupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("controller: error setting up controller with manager: %w", err)
	}

	// Start the manager with our reconciler in it
	go func() {
		err = mgr.Start(ctx)
		if err != nil {
			log.Fatalln("fatal controller error:", err)
		}
	}()

	return reconciler, nil
}
