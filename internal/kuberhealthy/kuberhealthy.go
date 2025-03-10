package kuberhealthy

import (
	"log"
)

// Kuberhealthy handles background processing for checks
type Kuberhealthy struct{}

func (kh *Kuberhealthy) StartCheck(namespace string, name string) {
	log.Println("Starting Kuberhealthy check", namespace, name)
	// Start background logic here
}

func (kh *Kuberhealthy) StopCheck(namespace string, name string) {
	log.Println("Stopping Kuberhealthy check", namespace, name)
	// Cleanup logic here
}
