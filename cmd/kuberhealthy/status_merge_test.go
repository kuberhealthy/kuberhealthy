package main

import (
	"testing"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestSetCheckStatusMergesAndClearsUUID verifies that setCheckStatus merges incoming
// fields without wiping timing fields, and that it clears the UUID to allow repeats.
func TestSetCheckStatusMergesAndClearsUUID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	t.Parallel()

	scheme := runtime.NewScheme()
	err := khapi.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	const (
		name      = "merge-check"
		namespace = "default"
	)
	originalLast := time.Now().Add(-2 * time.Minute).Unix()
	originalDur := 42 * time.Second

	existing := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: khapi.KuberhealthyCheckStatus{
			OK:                  false,
			Errors:              []string{"prior error"},
			ConsecutiveFailures: 2,
			LastRunDuration:     originalDur,
			Namespace:           "", // exercise defaulting path
			CurrentUUID:         "abc-123",
			LastRunUnix:         originalLast,
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(existing).WithStatusSubresource(existing).Build()

	incoming := &khapi.KuberhealthyCheckStatus{OK: true, Errors: []string{}, CurrentUUID: ""}
	err = setCheckStatus(cl, name, namespace, incoming)
	if err != nil {
		t.Fatalf("setCheckStatus returned error: %v", err)
	}

	var updated khapi.KuberhealthyCheck
	err = cl.Get(t.Context(), types.NamespacedName{Name: name, Namespace: namespace}, &updated)
	if err != nil {
		t.Fatalf("failed to get updated object: %v", err)
	}

	if updated.Status.CurrentUUID != "" {
		t.Fatalf("currentUUID not cleared, got %q", updated.Status.CurrentUUID)
	}
	if !updated.Status.OK || len(updated.Status.Errors) != 0 {
		t.Fatalf("status not merged correctly: %+v", updated.Status)
	}
	if updated.Status.LastRunUnix != originalLast {
		t.Fatalf("lastRunUnix was modified: got %d want %d", updated.Status.LastRunUnix, originalLast)
	}
	if updated.Status.LastRunDuration != originalDur {
		t.Fatalf("runDuration was modified: got %s want %s", updated.Status.LastRunDuration, originalDur)
	}
	if updated.Status.ConsecutiveFailures != 0 {
		t.Fatalf("consecutiveFailures not reset on OK, got %d", updated.Status.ConsecutiveFailures)
	}
	if updated.Status.Namespace != namespace {
		t.Fatalf("status namespace not defaulted, got %q want %q", updated.Status.Namespace, namespace)
	}
}
