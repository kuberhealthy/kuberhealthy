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
	"k8s.io/client-go/tools/record"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// waitForEvents drains the fake recorder until the expected number of events arrive or times out the test.
func waitForEvents(t *testing.T, events <-chan string, expected int) []string {
	t.Helper()

	collected := make([]string, 0, expected)
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()

	for len(collected) < expected {
		select {
		case event := <-events:
			collected = append(collected, event)
		case <-timeout.C:
			t.Fatalf("timed out waiting for %d events; received %d", expected, len(collected))
		}
	}

	return collected
}

// TestReaperTimesOutRunningPod ensures pods exceeding their timeout are deleted and the check is marked failed.
func TestReaperTimesOutRunningPod(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	lastRun := time.Now().Add(-time.Hour)
	now := time.Now()

	check := &khapi.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "timeout-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.HealthCheckStatus{
			LastRunUnix: lastRun.Unix(),
		},
	}
	check.SetCurrentUUID("abc123")
	check.SetOK()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "timeout-pod",
			Namespace:         "default",
			Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: check.CurrentUUID()},
			CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithRuntimeObjects(check, pod).
		WithStatusSubresource(check).
		Build()

	kh := New(context.Background(), cl)
	// Ensure completed pods are pruned immediately for this test.
	kh.ConfigureReaper(0, defaultMaxFailedPods, 0, 0)
	kh.Recorder = record.NewFakeRecorder(10)

	require.NoError(t, kh.reapOnce())

	// Pod should be deleted
	err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "timeout-pod"}, &corev1.Pod{})
	require.True(t, apierrors.IsNotFound(err))

	// Check status should remain untouched; timeout handling happens outside the reaper now.
	updated := &khapi.HealthCheck{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "timeout-check"}, updated))
	require.True(t, updated.Status.OK)
	require.Empty(t, updated.Status.Errors)
	require.Equal(t, "abc123", updated.CurrentUUID())

	events := waitForEvents(t, kh.Recorder.(*record.FakeRecorder).Events, 1)
	require.Contains(t, events[0], "CheckRunTimeout")
}

// TestReaperKeepsRunningPodsForFiveMinutes ensures running pods are retained for at least five minutes even if they exceed the timeout.
func TestReaperKeepsRunningPodsForFiveMinutes(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	lastRun := time.Now().Add(-2 * time.Minute)
	now := time.Now()

	check := &khapi.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "recent-running-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.HealthCheckStatus{
			LastRunUnix: lastRun.Unix(),
		},
	}
	check.SetCurrentUUID("abc123")
	check.SetOK()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "recent-running-pod",
			Namespace:         "default",
			Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: check.CurrentUUID()},
			CreationTimestamp: metav1.NewTime(now.Add(-2 * time.Minute)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithRuntimeObjects(check, pod).
		WithStatusSubresource(check).
		Build()

	kh := New(context.Background(), cl)
	// Keep the default failed pod limit while allowing running pods to age.
	kh.ConfigureReaper(0, defaultMaxFailedPods, 0, 0)
	kh.Recorder = record.NewFakeRecorder(10)

	require.NoError(t, kh.reapOnce())

	// Pod should still exist because less than five minutes have passed
	err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "recent-running-pod"}, &corev1.Pod{})
	require.NoError(t, err)

	updated := &khapi.HealthCheck{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "recent-running-check"}, updated))
	require.True(t, updated.Status.OK)
	require.Empty(t, updated.Status.Errors)
	require.Equal(t, "abc123", updated.CurrentUUID())

	select {
	case e := <-kh.Recorder.(*record.FakeRecorder).Events:
		t.Fatalf("unexpected event emitted: %s", e)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestReaperRemovesCompletedPods deletes completed pods after ten run intervals have passed.
func TestReaperRemovesCompletedPods(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping completed pod removal test in short mode")
	}
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	lastRun := time.Now().Add(-defaultRunInterval*10 - time.Minute)
	now := time.Now()

	check := &khapi.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "complete-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.HealthCheckStatus{
			LastRunUnix: lastRun.Unix(),
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "complete-pod",
			Namespace:         "default",
			Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "run-1"},
			CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithRuntimeObjects(check, pod).
		WithStatusSubresource(check).
		Build()

	kh := New(context.Background(), cl)
	// Configure immediate cleanup for completed pods.
	kh.ConfigureReaper(0, defaultMaxFailedPods, 0, 0)

	require.NoError(t, kh.reapOnce())

	// Pod should be deleted after grace period
	err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "complete-pod"}, &corev1.Pod{})
	require.True(t, apierrors.IsNotFound(err))
}

// TestReaperKeepsRecentCompletedPods retains completed pods that have not exceeded the grace period.
func TestReaperKeepsRecentCompletedPods(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping recent completion test in short mode")
	}
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	lastRun := time.Now().Add(-defaultRunInterval * 2)
	now := time.Now()

	check := &khapi.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "recent-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.HealthCheckStatus{
			LastRunUnix: lastRun.Unix(),
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "recent-pod",
			Namespace:         "default",
			Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "run-2"},
			CreationTimestamp: metav1.NewTime(now.Add(-time.Minute)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithRuntimeObjects(check, pod).
		WithStatusSubresource(check).
		Build()

	kh := New(context.Background(), cl)
	// Retain one completed pod so the recent run stays present.
	kh.ConfigureReaper(1, defaultMaxFailedPods, 0, 0)

	require.NoError(t, kh.reapOnce())

	// Pod should still exist because grace period has not elapsed
	err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "recent-pod"}, &corev1.Pod{})
	require.NoError(t, err)
}

// TestReaperPrunesFailedPods trims older failures once more than three exist.
func TestReaperPrunesFailedPods(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping prune test in short mode")
	}

	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	check := &khapi.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "failed-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.HealthCheckStatus{
			LastRunUnix: time.Now().Unix(),
		},
	}

	now := time.Now()
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "failed-oldest",
				Namespace:         "default",
				Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "run-a"},
				CreationTimestamp: metav1.NewTime(now.Add(-5 * time.Hour)),
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "failed-older",
				Namespace:         "default",
				Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "run-b"},
				CreationTimestamp: metav1.NewTime(now.Add(-4 * time.Hour)),
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "failed-middle",
				Namespace:         "default",
				Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "run-c"},
				CreationTimestamp: metav1.NewTime(now.Add(-3 * time.Hour)),
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "failed-newer",
				Namespace:         "default",
				Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "run-d"},
				CreationTimestamp: metav1.NewTime(now.Add(-2 * time.Hour)),
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "failed-newest",
				Namespace:         "default",
				Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "run-e"},
				CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Hour)),
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
	// Limit failed pods to three to exercise pruning behavior.
	kh.ConfigureReaper(0, 3, 0, 0)

	require.NoError(t, kh.reapOnce())

	var remaining corev1.PodList
	require.NoError(t, cl.List(context.Background(), &remaining, client.InNamespace("default"), client.HasLabels{runUUIDLabel}))
	var ours []corev1.Pod
	for i := range remaining.Items {
		if remaining.Items[i].Labels[primaryCheckLabel] == check.Name {
			ours = append(ours, remaining.Items[i])
		}
	}
	require.Len(t, ours, 3)
	var names []string
	for i := range ours {
		names = append(names, ours[i].Name)
	}
	require.ElementsMatch(t, []string{"failed-middle", "failed-newer", "failed-newest"}, names)
}

// TestReaperRetainsFailedPodsWithinLimit keeps failed pods when they are at or below the limit.
func TestReaperRetainsFailedPodsWithinLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retention test in short mode")
	}

	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	check := &khapi.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "retain-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.HealthCheckStatus{
			LastRunUnix: time.Now().Unix(),
		},
	}

	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failed-one",
				Namespace: "default",
				Labels:    map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "run-1"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failed-two",
				Namespace: "default",
				Labels:    map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "run-2"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failed-three",
				Namespace: "default",
				Labels:    map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "run-3"},
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
	// Keep failed pods when they are within the configured limit.
	kh.ConfigureReaper(0, 3, 0, 0)

	require.NoError(t, kh.reapOnce())

	var remaining corev1.PodList
	require.NoError(t, cl.List(context.Background(), &remaining, client.InNamespace("default"), client.HasLabels{runUUIDLabel}))
	var ours []corev1.Pod
	for i := range remaining.Items {
		if remaining.Items[i].Labels[primaryCheckLabel] == check.Name {
			ours = append(ours, remaining.Items[i])
		}
	}
	require.Len(t, ours, 3)
}

// TestReaperTreatsUnknownPhaseAsFailed ensures unexpected pod phases are trimmed with the failed-pod policy.
func TestReaperTreatsUnknownPhaseAsFailed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping unexpected phase test in short mode")
	}

	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	check := &khapi.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "odd-phase-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.HealthCheckStatus{
			LastRunUnix: time.Now().Unix(),
		},
	}

	now := time.Now()
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "mystery-1",
				Namespace:         "default",
				Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "m1"},
				CreationTimestamp: metav1.NewTime(now.Add(-4 * time.Hour)),
			},
			Status: corev1.PodStatus{Phase: corev1.PodPhase("MysteryPhase")},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "mystery-2",
				Namespace:         "default",
				Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "m2"},
				CreationTimestamp: metav1.NewTime(now.Add(-3 * time.Hour)),
			},
			Status: corev1.PodStatus{Phase: corev1.PodPhase("MysteryPhase")},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "mystery-3",
				Namespace:         "default",
				Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "m3"},
				CreationTimestamp: metav1.NewTime(now.Add(-2 * time.Hour)),
			},
			Status: corev1.PodStatus{Phase: corev1.PodPhase("MysteryPhase")},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "mystery-4",
				Namespace:         "default",
				Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "m4"},
				CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Hour)),
			},
			Status: corev1.PodStatus{Phase: corev1.PodPhase("MysteryPhase")},
		},
	}

	objs := []runtime.Object{check}
	for i := range pods {
		p := pods[i]
		objs = append(objs, &p)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)
	// Treat unknown phases as failures and apply the same retention limit.
	kh.ConfigureReaper(0, 3, 0, 0)

	require.NoError(t, kh.reapOnce())

	var remaining corev1.PodList
	require.NoError(t, cl.List(context.Background(), &remaining, client.InNamespace("default"), client.HasLabels{runUUIDLabel}))
	var ours []corev1.Pod
	for i := range remaining.Items {
		if remaining.Items[i].Labels[primaryCheckLabel] == check.Name {
			ours = append(ours, remaining.Items[i])
		}
	}
	require.Len(t, ours, 3)
	var names []string
	for i := range ours {
		names = append(names, ours[i].Name)
	}
	require.ElementsMatch(t, []string{"mystery-2", "mystery-3", "mystery-4"}, names)
}

// TestReaperDropsFailedPodsByAge ensures retention windows remove old failed pods.
func TestReaperDropsFailedPodsByAge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping failed pod age test in short mode")
	}

	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	check := &khapi.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "failed-age-check",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Status: khapi.HealthCheckStatus{
			LastRunUnix: time.Now().Unix(),
		},
	}

	now := time.Now()
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "failed-old",
				Namespace:         "default",
				Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "old"},
				CreationTimestamp: metav1.NewTime(now.Add(-48 * time.Hour)),
			},
			Status: corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "failed-new",
				Namespace:         "default",
				Labels:            map[string]string{primaryCheckLabel: check.Name, runUUIDLabel: "new"},
				CreationTimestamp: metav1.NewTime(now.Add(-12 * time.Hour)),
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
	// Drop failed pods older than one day, even when under the count limit.
	kh.ConfigureReaper(0, 3, 1, 0)

	require.NoError(t, kh.reapOnce())

	var remaining corev1.PodList
	require.NoError(t, cl.List(context.Background(), &remaining, client.InNamespace("default"), client.HasLabels{runUUIDLabel}))
	var ours []corev1.Pod
	for i := range remaining.Items {
		if remaining.Items[i].Labels[primaryCheckLabel] == check.Name {
			ours = append(ours, remaining.Items[i])
		}
	}
	require.Len(t, ours, 1)
	require.Equal(t, "failed-new", ours[0].Name)
}
