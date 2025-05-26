package kuberhealthy

import (
	"context"
	"fmt"
	"log"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	client "sigs.k8s.io/controller-runtime/pkg/client"
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
func (kh *Kuberhealthy) StartCheck(client client.Client, khcheck *khcrdsv2.KuberhealthyCheck) error {
	log.Println("Starting Kuberhealthy check", khcheck.GetNamespace(), khcheck.GetName())
	// Start background logic here
	return nil
}

// StartCheck stops tracking and managing a khcheck
func (kh *Kuberhealthy) StopCheck(client client.Client, khcheck *khcrdsv2.KuberhealthyCheck) error {
	log.Println("Stopping Kuberhealthy check", khcheck.GetNamespace(), khcheck.GetName())
	// Cleanup logic here
	return nil
}

// Start starts a new Kuberhealthy manager (this is the thing that kubebuilder makes)
// along with other various processes needed to manager Kuberhealthy checks.
func (kh *Kuberhealthy) Start(ctx context.Context) error {
	if kh.IsStarted() {
		return fmt.Errorf("error: kuberhealthy main controller was started but it was already running")
	}
	log.Println("Kuberhealthy start")
	return nil
}

// IsStarted returns if this instance is running or not
func (kh *Kuberhealthy) IsStarted() bool {
	return kh.Running
}
