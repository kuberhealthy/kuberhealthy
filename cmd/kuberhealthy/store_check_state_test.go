package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/controller"
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

func (c *conflictClient) Status() client.StatusWriter {
	return &conflictStatusWriter{StatusWriter: c.Client.Status(), parent: c}
}

type conflictStatusWriter struct {
	client.StatusWriter
	parent *conflictClient
}

func (w *conflictStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	w.parent.updateCalls++
	if w.parent.updateCalls == 1 {
		return fmt.Errorf("the object has been modified")
	}
	return w.StatusWriter.Update(ctx, obj, opts...)
}

func TestStoreCheckStateRetriesOnConflict(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := khapi.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}
	existing := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "conflict-check",
			Namespace: "default",
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).WithStatusSubresource(existing).Build()
	cc := &conflictClient{Client: fakeClient}

	origController := KHController
	t.Cleanup(func() { KHController = origController })
	KHController = &controller.KHCheckController{Client: cc}

	status := &khapi.KuberhealthyCheckStatus{OK: true}
	if err := storeCheckState("conflict-check", "default", status); err != nil {
		t.Fatalf("storeCheckState returned error: %v", err)
	}
	if cc.updateCalls < 2 {
		t.Fatalf("expected at least 2 update calls, got %d", cc.updateCalls)
	}

	var updated khapi.KuberhealthyCheck
	if err := cc.Get(context.Background(), types.NamespacedName{Name: "conflict-check", Namespace: "default"}, &updated); err != nil {
		t.Fatalf("failed to get updated khcheck: %v", err)
	}
	if !updated.Status.OK {
		t.Fatalf("expected status OK true, got false")
	}
}
