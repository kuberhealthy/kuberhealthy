package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
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
		return apierrors.NewConflict(khapi.GroupVersion.WithResource("healthchecks").GroupResource(), obj.GetName(), fmt.Errorf("the object has been modified"))
	}
	return w.StatusWriter.Update(ctx, obj, opts...)
}

// TestStoreCheckStateRetriesOnConflict verifies that storeCheckState retries updates when a conflict occurs.
func TestStoreCheckStateRetriesOnConflict(t *testing.T) {
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
		t.Fatalf("failed to get updated healthcheck: %v", err)
	}
	if !updated.Status.OK {
		t.Fatalf("expected status OK true, got false")
	}
}

type failingClient struct {
	client.Client
	err         error
	updateCalls int
}

// Status wraps the underlying status writer so every update fails with err.
func (c *failingClient) Status() client.StatusWriter {
	return &failingStatusWriter{StatusWriter: c.Client.Status(), parent: c}
}

type failingStatusWriter struct {
	client.StatusWriter
	parent *failingClient
}

// Update always returns the parent's error.
func (w *failingStatusWriter) Update(context.Context, client.Object, ...client.SubResourceUpdateOption) error {
	w.parent.updateCalls++
	return w.parent.err
}

// TestStoreCheckStateRetryLimits verifies that only conflicts are retried and
// that a conflict which never clears is given up on.
func TestStoreCheckStateRetryLimits(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := khapi.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}
	gr := khapi.GroupVersion.WithResource("healthchecks").GroupResource()

	cases := []struct {
		name      string
		err       error
		wantCalls int
	}{
		{name: "other error is not retried", err: apierrors.NewForbidden(gr, "limit-check", fmt.Errorf("nope")), wantCalls: 1},
		{name: "persistent conflict gives up", err: apierrors.NewConflict(gr, "limit-check", fmt.Errorf("the object has been modified")), wantCalls: retry.DefaultRetry.Steps},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			existing := &khapi.HealthCheck{ObjectMeta: metav1.ObjectMeta{Name: "limit-check", Namespace: "default"}}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).WithStatusSubresource(existing).Build()
			fc := &failingClient{Client: fakeClient, err: tc.err}

			err := storeCheckState(fc, "limit-check", "default", &khapi.HealthCheckStatus{OK: true})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected the update error to be wrapped, got %v", err)
			}
			if fc.updateCalls != tc.wantCalls {
				t.Fatalf("expected %d update calls, got %d", tc.wantCalls, fc.updateCalls)
			}
		})
	}
}
