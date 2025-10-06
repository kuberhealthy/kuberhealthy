package kuberhealthy

import (
	"context"
	"fmt"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
)

const khCheckFinalizer = "kuberhealthy.io/healthcheck"

// hasFinalizer returns true when the check contains the kuberhealthy healthcheck finalizer.
func (k *Kuberhealthy) hasFinalizer(check *khapi.HealthCheck) bool {
	// iterate over the finalizers and return when the kuberhealthy finalizer is present
	for _, f := range check.Finalizers {
		if f == khCheckFinalizer {
			return true
		}
	}
	return false
}

// addFinalizer appends the kuberhealthy finalizer if it is missing.
func (k *Kuberhealthy) addFinalizer(ctx context.Context, check *khapi.HealthCheck) error {
	if k.hasFinalizer(check) {
		return nil
	}
	// append the kuberhealthy healthcheck finalizer so we control resource cleanup
	check.Finalizers = append(check.Finalizers, khCheckFinalizer)
	err := k.CheckClient.Update(ctx, check)
	if err != nil {
		return fmt.Errorf("failed to add finalizer: %w", err)
	}
	return nil
}

// deleteFinalizer removes the kuberhealthy finalizer when present.
func (k *Kuberhealthy) deleteFinalizer(ctx context.Context, check *khapi.HealthCheck) error {
	if !k.hasFinalizer(check) {
		return nil
	}
	// build a new finalizer slice that omits the kuberhealthy finalizer
	var finalizers []string
	for _, f := range check.Finalizers {
		if f != khCheckFinalizer {
			finalizers = append(finalizers, f)
		}
	}
	check.Finalizers = finalizers
	// persist the updated finalizers back to the API server
	err := k.CheckClient.Update(ctx, check)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}
	return nil
}
