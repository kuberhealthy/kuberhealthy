package kuberhealthy

import (
	"context"
	"strings"
	"testing"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCheckPodSpec(t *testing.T) {
	kh := New(context.Background(), nil)

	check := &khcrdsv2.KuberhealthyCheck{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kuberhealthy.github.io/v2",
			Kind:       "KuberhealthyCheck",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-check",
			Namespace: "example-ns",
			UID:       types.UID("abc123"),
		},
		Spec: khcrdsv2.KuberhealthyCheckSpec{
			PodSpec: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "test",
						Image: "busybox",
					}},
				},
			},
		},
	}

	pod := kh.CheckPodSpec(check)

	require.Equal(t, check.Namespace, pod.Namespace)
	require.True(t, strings.HasPrefix(pod.Name, check.Name+"-"))
	require.Equal(t, check.Spec.PodSpec.Spec, pod.Spec)

	require.Equal(t, "kuberhealthy", pod.Annotations["createdBy"])
	require.Equal(t, check.Name, pod.Annotations["kuberhealthyCheckName"])
	require.NotEmpty(t, pod.Annotations["createdTime"])

	require.Equal(t, check.Name, pod.Labels["khcheck"])

	require.Len(t, pod.OwnerReferences, 1)
	owner := pod.OwnerReferences[0]
	require.Equal(t, check.Name, owner.Name)
	require.Equal(t, check.UID, owner.UID)
	require.NotNil(t, owner.Controller)
	require.True(t, *owner.Controller)
}

func TestIsStarted(t *testing.T) {
	kh := &Kuberhealthy{Running: true}
	require.True(t, kh.IsStarted())
	kh.Running = false
	require.False(t, kh.IsStarted())
}

func TestSetAndGetCheckPodName(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, khcrdsv2.AddToScheme(scheme))

	check := &khcrdsv2.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)

	nn := types.NamespacedName{Namespace: check.Namespace, Name: check.Name}

	require.NoError(t, kh.setCheckPodName(nn, "pod-123"))

	name, err := kh.getCurrentPodName(check)
	require.NoError(t, err)
	require.Equal(t, "pod-123", name)
}

func TestSetFreshUUID(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, khcrdsv2.AddToScheme(scheme))

	check := &khcrdsv2.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "uuid-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)

	nn := types.NamespacedName{Namespace: check.Namespace, Name: check.Name}
	require.NoError(t, kh.setFreshUUID(nn))

	fetched, err := kh.getCheck(nn)
	require.NoError(t, err)
	require.NotEmpty(t, fetched.Status.CurrentUUID)
}
