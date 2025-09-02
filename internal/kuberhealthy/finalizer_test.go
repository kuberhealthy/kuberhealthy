package kuberhealthy

import (
	"context"
	"testing"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAddFinalizer(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "add-finalizer",
			Namespace:       "default",
			ResourceVersion: "1",
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).Build()
	kh := New(context.Background(), cl)
	require.NoError(t, kh.addFinalizer(context.Background(), check))
	fetched := &khapi.KuberhealthyCheck{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "add-finalizer", Namespace: "default"}, fetched))
	require.Contains(t, fetched.Finalizers, khCheckFinalizer)
}

func TestHandleUpdateRemovesFinalizer(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "remove-finalizer",
			Namespace:       "default",
			Finalizers:      []string{khCheckFinalizer},
			ResourceVersion: "1",
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)
	oldU := &unstructured.Unstructured{}
	oldU.Object, _ = runtime.DefaultUnstructuredConverter.ToUnstructured(check)
	now := metav1.Now()
	deleting := check.DeepCopy()
	deleting.DeletionTimestamp = &now
	newU := &unstructured.Unstructured{}
	newU.Object, _ = runtime.DefaultUnstructuredConverter.ToUnstructured(deleting)
	kh.handleUpdate(oldU, newU)
	fetched := &khapi.KuberhealthyCheck{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "remove-finalizer", Namespace: "default"}, fetched))
	require.NotContains(t, fetched.Finalizers, khCheckFinalizer)
}

func TestHandleDeleteRemovesFinalizer(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	now := metav1.Now()
	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "delete-finalizer",
			Namespace:         "default",
			Finalizers:        []string{khCheckFinalizer},
			ResourceVersion:   "1",
			DeletionTimestamp: &now,
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)
	u := &unstructured.Unstructured{}
	u.Object, _ = runtime.DefaultUnstructuredConverter.ToUnstructured(check)
	kh.handleDelete(u)
	fetched := &khapi.KuberhealthyCheck{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: "delete-finalizer", Namespace: "default"}, fetched)
	if apierrors.IsNotFound(err) {
		return
	}
	require.NoError(t, err)
	require.NotContains(t, fetched.Finalizers, khCheckFinalizer)
}
