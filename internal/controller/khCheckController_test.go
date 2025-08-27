package controller

import (
	"context"
	"fmt"
	"testing"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// conflictClient simulates a conflict on the first status update call.
type conflictClient struct {
	client.Client
	calls int
}

func (c *conflictClient) Status() client.StatusWriter {
	return &conflictStatusWriter{StatusWriter: c.Client.Status(), parent: c}
}

type conflictStatusWriter struct {
	client.StatusWriter
	parent *conflictClient
}

func (w *conflictStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	w.parent.calls++
	if w.parent.calls == 1 {
		return apierrors.NewConflict(schema.GroupResource{Group: "kuberhealthy.kuberhealthy.github.io", Resource: "kuberhealthychecks"}, obj.GetName(), fmt.Errorf("conflict"))
	}
	return w.StatusWriter.Update(ctx, obj, opts...)
}

func TestEnqueueAddsRequest(t *testing.T) {
	t.Parallel()
	c := &KHCheckController{queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())}
	obj := &khcrdsv2.KuberhealthyCheck{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"}}
	c.enqueue(obj)
	if c.queue.Len() != 1 {
		t.Fatalf("expected queue length 1 got %d", c.queue.Len())
	}
}

func TestReconcileRetriesOnConflict(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	if err := khcrdsv2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}
	check := &khcrdsv2.KuberhealthyCheck{ObjectMeta: metav1.ObjectMeta{Name: "conflict", Namespace: "ns"}}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(check).WithStatusSubresource(check).Build()
	cc := &conflictClient{Client: fakeClient}
	controller := &KHCheckController{Client: cc}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "conflict", Namespace: "ns"}}
	if _, err := controller.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if cc.calls < 2 {
		t.Fatalf("expected at least 2 status update calls, got %d", cc.calls)
	}
}

func TestSanitizeCheck(t *testing.T) {
	t.Parallel()
	check := &khcrdsv2.KuberhealthyCheck{ObjectMeta: metav1.ObjectMeta{UID: "abc", ResourceVersion: "1", ManagedFields: []metav1.ManagedFieldsEntry{{}}}}
	sanitizeCheck(check)
	if check.ObjectMeta.UID != "" || check.ObjectMeta.ManagedFields != nil {
		t.Fatalf("sanitizeCheck did not clear metadata fields")
	}
}
