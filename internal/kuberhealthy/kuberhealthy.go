package kuberhealthy

import (
	"context"
	"log"
)

// Kuberhealthy handles background processing for checks
type Kuberhealthy struct {
	Context context.Context
}

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

	return nil
}
