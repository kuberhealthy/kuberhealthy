package kuberhealthy

import (
	"context"
	"testing"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestReaperTimesOutRunningPod ensures pods exceeding their timeout are deleted and the check is marked failed.
func TestReaperTimesOutRunningPod(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	lastRun := time.Now().Add(-time.Hour)

	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "timeout-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.KuberhealthyCheckStatus{
			LastRunUnix: lastRun.Unix(),
		},
	}
	check.SetCurrentUUID("abc123")
	check.SetOK()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "timeout-pod",
			Namespace: "default",
			Labels:    map[string]string{checkLabel: check.Name, runUUIDLabel: check.CurrentUUID()},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithRuntimeObjects(check, pod).
		WithStatusSubresource(check).
		Build()

	kh := New(context.Background(), cl)

	require.NoError(t, kh.reapOnce())

	// Pod should be deleted
	err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "timeout-pod"}, &corev1.Pod{})
	require.True(t, apierrors.IsNotFound(err))

	// Check status should show timeout
	updated := &khapi.KuberhealthyCheck{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "timeout-check"}, updated))
	require.False(t, updated.Status.OK)
	require.Len(t, updated.Status.Errors, 1)
	require.Contains(t, updated.Status.Errors[0], "timed out")
	require.Empty(t, updated.CurrentUUID())
}

// TestReaperRemovesCompletedPods deletes completed pods after three run intervals have passed.
func TestReaperRemovesCompletedPods(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	lastRun := time.Now().Add(-defaultRunInterval*3 - time.Minute)

	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "complete-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.KuberhealthyCheckStatus{
			LastRunUnix: lastRun.Unix(),
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "complete-pod",
			Namespace: "default",
			Labels:    map[string]string{checkLabel: check.Name, runUUIDLabel: "run-1"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithRuntimeObjects(check, pod).
		WithStatusSubresource(check).
		Build()

	kh := New(context.Background(), cl)

	require.NoError(t, kh.reapOnce())

	// Pod should be deleted after grace period
	err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "complete-pod"}, &corev1.Pod{})
	require.True(t, apierrors.IsNotFound(err))
}

// TestReaperKeepsRecentCompletedPods retains completed pods that have not exceeded the grace period.
func TestReaperKeepsRecentCompletedPods(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	lastRun := time.Now().Add(-defaultRunInterval * 2)

	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "recent-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.KuberhealthyCheckStatus{
			LastRunUnix: lastRun.Unix(),
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "recent-pod",
			Namespace: "default",
			Labels:    map[string]string{checkLabel: check.Name, runUUIDLabel: "run-2"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithRuntimeObjects(check, pod).
		WithStatusSubresource(check).
		Build()

	kh := New(context.Background(), cl)

	require.NoError(t, kh.reapOnce())

	// Pod should still exist because grace period has not elapsed
	err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "recent-pod"}, &corev1.Pod{})
	require.NoError(t, err)
}

// TestReaperPrunesFailedPods prunes failed pods based on retention days and maximum count.
func TestReaperPrunesFailedPods(t *testing.T) {
	t.Setenv("KH_ERROR_POD_RETENTION_DAYS", "2")
	t.Setenv("KH_MAX_ERROR_POD_COUNT", "2")

	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "failed-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.KuberhealthyCheckStatus{
			LastRunUnix: time.Now().Unix(),
		},
	}

	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failed-oldest",
				Namespace: "default",
				Labels:    map[string]string{checkLabel: check.Name, runUUIDLabel: "run-a"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failed-middle",
				Namespace: "default",
				Labels:    map[string]string{checkLabel: check.Name, runUUIDLabel: "run-b"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failed-newest",
				Namespace: "default",
				Labels:    map[string]string{checkLabel: check.Name, runUUIDLabel: "run-c"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
	}

	objs := []runtime.Object{check}
	for i := range pods {
		p := pods[i]
		objs = append(objs, &p)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)

	require.NoError(t, kh.reapOnce())

	var remaining corev1.PodList
	require.NoError(t, cl.List(context.Background(), &remaining, client.InNamespace("default"), client.HasLabels{runUUIDLabel}))
	var ours []corev1.Pod
	for i := range remaining.Items {
		if remaining.Items[i].Labels[checkLabel] == check.Name {
			ours = append(ours, remaining.Items[i])
		}
	}
	require.Len(t, ours, 2)
}

// TestReaperRetainsFailedPodsWithinRetention keeps failed pods when they fall within retention limits.
func TestReaperRetainsFailedPodsWithinRetention(t *testing.T) {
	t.Setenv("KH_ERROR_POD_RETENTION_DAYS", "2")
	t.Setenv("KH_MAX_ERROR_POD_COUNT", "3")

	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "retain-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.KuberhealthyCheckStatus{
			LastRunUnix: time.Now().Unix(),
		},
	}

	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failed-one",
				Namespace: "default",
				Labels:    map[string]string{checkLabel: check.Name, runUUIDLabel: "run-1"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failed-two",
				Namespace: "default",
				Labels:    map[string]string{checkLabel: check.Name, runUUIDLabel: "run-2"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
	}

	objs := []runtime.Object{check}
	for i := range pods {
		p := pods[i]
		objs = append(objs, &p)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)

	require.NoError(t, kh.reapOnce())

	var remaining corev1.PodList
	require.NoError(t, cl.List(context.Background(), &remaining, client.InNamespace("default"), client.HasLabels{runUUIDLabel}))
	var ours []corev1.Pod
	for i := range remaining.Items {
		if remaining.Items[i].Labels[checkLabel] == check.Name {
			ours = append(ours, remaining.Items[i])
		}
	}
	require.Len(t, ours, 2)
}

// TestReaperDeletesFailedPodsPastRetention removes failed pods older than the configured retention period.
func TestReaperDeletesFailedPodsPastRetention(t *testing.T) {
	t.Setenv("KH_ERROR_POD_RETENTION_DAYS", "1")
	t.Setenv("KH_MAX_ERROR_POD_COUNT", "5")

	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "old-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.KuberhealthyCheckStatus{
			LastRunUnix: time.Now().Add(-26 * time.Hour).Unix(),
		},
	}

	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failed-oldest",
				Namespace: "default",
				Labels:    map[string]string{checkLabel: check.Name, runUUIDLabel: "old-1"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failed-older",
				Namespace: "default",
				Labels:    map[string]string{checkLabel: check.Name, runUUIDLabel: "old-2"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
	}

	objs := []runtime.Object{check}
	for i := range pods {
		p := pods[i]
		objs = append(objs, &p)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)

	require.NoError(t, kh.reapOnce())

	var remaining corev1.PodList
	require.NoError(t, cl.List(context.Background(), &remaining, client.InNamespace("default"), client.HasLabels{runUUIDLabel}))
	var ours []corev1.Pod
	for i := range remaining.Items {
		if remaining.Items[i].Labels[checkLabel] == check.Name {
			ours = append(ours, remaining.Items[i])
		}
	}
	require.Len(t, ours, 0)
}
