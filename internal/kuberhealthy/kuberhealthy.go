package kuberhealthy

import (
	"context"
	"fmt"
	"log"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Kuberhealthy handles background processing for checks
type Kuberhealthy struct{}

// New creates a new Kuberhealthy instance
func New() (*Kuberhealthy, error) {
	return &Kuberhealthy{}, nil
}

// StartCheck begins tracking and managing a khcheck
func (kh *Kuberhealthy) StartCheck(namespace string, name string) {
	log.Println("Starting Kuberhealthy check", namespace, name)
	// Start background logic here
}

// StartCheck stops tracking and managing a khcheck
func (kh *Kuberhealthy) StopCheck(namespace string, name string) {
	log.Println("Stopping Kuberhealthy check", namespace, name)
	// Cleanup logic here
}

// Start starts a new Kuberhealthy manager (this is the thing that kubebuilder makes)
// along with other various processes needed to manager Kuberhealthy checks.
func (kh *Kuberhealthy) Start(ctx context.Context) error {
	// Create a new manager from kubernetes
	mgr, err := ctrl.NewManager(config.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		return fmt.Errorf("kuberhealthy: failed to create manager:", err)
	}

	// Initialize and register the kuberhealthy controller
	khController, err := khcontroller.New(mgr, kh)
	if err != nil {
		return fmt.Errorf("kuberhealthy: failed to register controller with manager:", err)
	}

	// Start the manager in a goroutine
	go func() {
		log.Infoln("kuberhealthy: Starting manager...")
		err := mgr.Start(ctx)
		if err != nil {
			return fmt.Errorf("kuberhealthy: Manager stopped with error:", err)
		}
	}()

	return nil
}
