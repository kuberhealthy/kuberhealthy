package kuberhealthy

import (
	"context"
	"fmt"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
)

const khCheckFinalizer = "kuberhealthy.io/kuberhealthycheck"

// hasFinalizer returns true when the check contains the kuberhealthy finalizer.
func (k *Kuberhealthy) hasFinalizer(check *khapi.KuberhealthyCheck) bool {
	for _, f := range check.Finalizers {
		if f == khCheckFinalizer {
			return true
		}
	}
	return false
}

// addFinalizer appends the kuberhealthy finalizer if it is missing.
func (k *Kuberhealthy) addFinalizer(ctx context.Context, check *khapi.KuberhealthyCheck) error {
	if k.hasFinalizer(check) {
		return nil
	}
	check.Finalizers = append(check.Finalizers, khCheckFinalizer)
	if err := k.CheckClient.Update(ctx, check); err != nil {
		return fmt.Errorf("failed to add finalizer: %w", err)
	}
	return nil
}

// deleteFinalizer removes the kuberhealthy finalizer when present.
func (k *Kuberhealthy) deleteFinalizer(ctx context.Context, check *khapi.KuberhealthyCheck) error {
	if !k.hasFinalizer(check) {
		return nil
	}
	var finalizers []string
	for _, f := range check.Finalizers {
		if f != khCheckFinalizer {
			finalizers = append(finalizers, f)
		}
	}
	check.Finalizers = finalizers
	if err := k.CheckClient.Update(ctx, check); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}
	return nil
}
