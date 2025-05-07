package kuberhealthy

import (
	"context"
	"log"
)

// Kuberhealthy handles background processing for checks
type Kuberhealthy struct {
	Context context.Context
	Running bool // indicates that Start() has been called and this instance is running
}

// New creates a new Kuberhealthy instance
func New(ctx context.Context) (*Kuberhealthy, error) {
	log.Println("New Kuberhealthy created")
	return &Kuberhealthy{
		Context: ctx,
	}, nil
}

// StartCheck begins tracking and managing a khcheck
func (kh *Kuberhealthy) StartCheck(namespace string, name string) error {
	log.Println("Starting Kuberhealthy check", namespace, name)
	// Start background logic here
	return nil
}

// StartCheck stops tracking and managing a khcheck
func (kh *Kuberhealthy) StopCheck(namespace string, name string) error {
	log.Println("Stopping Kuberhealthy check", namespace, name)
	// Cleanup logic here
	return nil
}

// Start starts a new Kuberhealthy manager (this is the thing that kubebuilder makes)
// along with other various processes needed to manager Kuberhealthy checks.
func (kh *Kuberhealthy) Start(ctx context.Context) error {
	log.Println("Kuberhealthy start")
	return nil
}

// IsStarted returns if this instance is running or not
func (kh *Kuberhealthy) IsStarted() bool {
	return kh.Running
}
