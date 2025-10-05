package main

import (
	"context"
	"fmt"
	"testing"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type conflictClient struct {
	client.Client
	updateCalls int
}

// Status wraps the underlying status writer so the test can inject a conflict.
func (c *conflictClient) Status() client.StatusWriter {
	return &conflictStatusWriter{StatusWriter: c.Client.Status(), parent: c}
}

type conflictStatusWriter struct {
	client.StatusWriter
	parent *conflictClient
}

// Update returns a conflict error on the first call and delegates to the embedded writer afterward.
func (w *conflictStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	w.parent.updateCalls++
	if w.parent.updateCalls == 1 {
		// simulate resource version conflicts so storeCheckState must retry
		return fmt.Errorf("the object has been modified")
	}
	return w.StatusWriter.Update(ctx, obj, opts...)
}

// TestStoreCheckStateRetriesOnConflict verifies that storeCheckState retries updates when a conflict occurs.
func TestStoreCheckStateRetriesOnConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	scheme := runtime.NewScheme()
	err := khapi.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}
	existing := &khapi.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "conflict-check",
			Namespace: "default",
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).WithStatusSubresource(existing).Build()
	cc := &conflictClient{Client: fakeClient}

	status := &khapi.HealthCheckStatus{OK: true}
	err = storeCheckState(cc, "conflict-check", "default", status)
	if err != nil {
		t.Fatalf("storeCheckState returned error: %v", err)
	}
	if cc.updateCalls < 2 {
		t.Fatalf("expected at least 2 update calls, got %d", cc.updateCalls)
	}

	var updated khapi.HealthCheck
	err = cc.Get(context.Background(), types.NamespacedName{Name: "conflict-check", Namespace: "default"}, &updated)
	if err != nil {
		t.Fatalf("failed to get updated khcheck: %v", err)
	}
	if !updated.Status.OK {
		t.Fatalf("expected status OK true, got false")
	}
}
