package kuberhealthy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/envs"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestCheckPodSpec builds a pod from a check and asserts metadata and ownership are populated.
func TestCheckPodSpec(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))

	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-check",
			Namespace: "example-ns",
			UID:       types.UID("abc123"),
		},
		Spec: khapi.KuberhealthyCheckSpec{
			ExtraLabels:      map[string]string{"extra": "label"},
			ExtraAnnotations: map[string]string{"anno": "value"},
			PodSpec: khapi.CheckPodSpec{
				Metadata: &khapi.CheckPodMetadata{
					Labels:      map[string]string{"metaLabel": "metaVal"},
					Annotations: map[string]string{"metaAnno": "metaVal"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "test",
						Image: "busybox",
					}},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)
	kh.SetReportingURL("http://example.com/check")

	pod := kh.CheckPodSpec(check)

	uuid, err := kh.getCurrentUUID(types.NamespacedName{Namespace: check.Namespace, Name: check.Name})
	require.NoError(t, err)

	require.Equal(t, check.Namespace, pod.Namespace)
	require.True(t, strings.HasPrefix(pod.Name, check.Name+"-"))
	require.Equal(t, check.Spec.PodSpec.Spec.RestartPolicy, pod.Spec.RestartPolicy)
	require.Len(t, pod.Spec.Containers, 1)
	c := pod.Spec.Containers[0]
	require.Equal(t, check.Spec.PodSpec.Spec.Containers[0].Image, c.Image)
	requireEnvVar(t, c.Env, envs.KHReportingURL, kh.ReportingURL)
	requireEnvVar(t, c.Env, envs.KHRunUUID, uuid)

	require.Equal(t, "kuberhealthy", pod.Annotations["createdBy"])
	require.Equal(t, uuid, pod.Annotations[runUUIDLabel])
	require.NotEmpty(t, pod.Annotations["createdTime"])
	require.Equal(t, check.Name, pod.Annotations["kuberhealthyCheckName"])

	require.Equal(t, check.Name, pod.Labels[checkLabel])
	require.Equal(t, uuid, pod.Labels[runUUIDLabel])
	require.Equal(t, "label", pod.Labels["extra"])
	require.Equal(t, "metaVal", pod.Labels["metaLabel"])
	require.Equal(t, "value", pod.Annotations["anno"])
	require.Equal(t, "metaVal", pod.Annotations["metaAnno"])

	require.Len(t, pod.OwnerReferences, 1)
	owner := pod.OwnerReferences[0]
	require.Equal(t, check.Name, owner.Name)
	require.Equal(t, check.UID, owner.UID)
	require.Equal(t, khapi.GroupVersion.String(), owner.APIVersion)
	require.Equal(t, "KuberhealthyCheck", owner.Kind)
	require.NotNil(t, owner.Controller)
	require.True(t, *owner.Controller)
	require.NotNil(t, owner.BlockOwnerDeletion)
	require.True(t, *owner.BlockOwnerDeletion)
}

func requireEnvVar(t *testing.T, env []corev1.EnvVar, name, val string) {
	t.Helper()
	for i := range env {
		if env[i].Name == name {
			require.Equal(t, val, env[i].Value)
			return
		}
	}
	t.Fatalf("env var %s not set", name)
}

// TestIsStarted verifies that IsStarted reflects the running state of Kuberhealthy.
func TestIsStarted(t *testing.T) {
	t.Parallel()
	kh := &Kuberhealthy{running: true}
	require.True(t, kh.IsStarted())
	kh.running = false
	require.False(t, kh.IsStarted())
}

// TestSetFreshUUID ensures a new UUID is written to the check status.
func TestSetFreshUUID(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))

	check := &khapi.KuberhealthyCheck{
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

	fetched, err := kh.readCheck(nn)
	require.NoError(t, err)
	require.NotEmpty(t, fetched.CurrentUUID())
}

// TestStartCheckCreatesPodInCheckNamespace verifies StartCheck creates a pod in the same namespace as the check.
func TestStartCheckCreatesPodInCheckNamespace(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ns-check",
			Namespace: "check-ns",
		},
		Spec: khapi.KuberhealthyCheckSpec{
			PodSpec: khapi.CheckPodSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "test",
						Image: "busybox",
					}},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)
	kh.SetReportingURL("http://example.com")

	require.NoError(t, kh.StartCheck(check))

	pods := &corev1.PodList{}
	require.NoError(t, cl.List(context.Background(), pods))
	require.Len(t, pods.Items, 1)
	require.Equal(t, check.Namespace, pods.Items[0].Namespace)
}

// TestStartCheckPodCreationFailureClearsRunState ensures that a pod creation failure marks the check as failed and
// leaves it ready for the next scheduler attempt.
func TestStartCheckPodCreationFailureClearsRunState(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "create-failure-check",
			Namespace: "default",
		},
		Spec: khapi.KuberhealthyCheckSpec{
			PodSpec: khapi.CheckPodSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "test",
						Image: "busybox",
					}},
				},
			},
		},
	}

	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	client := &toggleCreateClient{Client: baseClient, failCreates: true}
	kh := New(context.Background(), client)
	kh.SetReportingURL("http://example.com")

	err := kh.StartCheck(check)
	require.Error(t, err)

	namespacedName := types.NamespacedName{Namespace: check.Namespace, Name: check.Name}
	stored, getErr := khapi.GetCheck(context.Background(), client, namespacedName)
	require.NoError(t, getErr)
	require.Empty(t, stored.CurrentUUID())
	require.False(t, stored.Status.OK)
	require.Len(t, stored.Status.Errors, 1)
	require.Contains(t, stored.Status.Errors[0], "failed to create check pod")

	client.failCreates = false
	refreshed, getErr := khapi.GetCheck(context.Background(), client, namespacedName)
	require.NoError(t, getErr)
	require.NoError(t, kh.StartCheck(refreshed))

	postRun, getErr := khapi.GetCheck(context.Background(), client, namespacedName)
	require.NoError(t, getErr)
	require.NotEmpty(t, postRun.CurrentUUID())
}

// toggleCreateClient wraps a controller-runtime client and injects pod creation failures when requested.
type toggleCreateClient struct {
	client.Client
	failCreates bool
}

// Create overrides the embedded client's behavior, returning an injected error for pods when failCreates is true.
func (c *toggleCreateClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if c.failCreates {
		if _, ok := obj.(*corev1.Pod); ok {
			return errors.New("injected pod creation failure")
		}
	}
	return c.Client.Create(ctx, obj, opts...)
}

// TestScheduleStartsCheck confirms that scheduleChecks triggers a run when a check is due.
func TestScheduleStartsCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping schedule check test in short mode")
	}
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	check := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kuberhealthy.github.io/v2",
		"kind":       "KuberhealthyCheck",
		"metadata": map[string]interface{}{
			"name":            "sched-check",
			"namespace":       "default",
			"resourceVersion": "1",
		},
		"spec": map[string]interface{}{
			"runInterval": "1s",
			"podSpec": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "test", "image": "busybox"},
					},
				},
			},
		},
	}}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)

	kh.scheduleChecks()

	fetched := &khapi.KuberhealthyCheck{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "sched-check", Namespace: "default"}, fetched))
	require.NotEmpty(t, fetched.CurrentUUID())
	require.NotZero(t, fetched.Status.LastRunUnix)
}

// TestCheckRunTimesOutAfterDeadline ensures the timeout watcher marks checks as failed after the grace period.
func TestCheckRunTimesOutAfterDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout watcher test in short mode")
	}

	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	timeout := 500 * time.Millisecond
	start := time.Now()

	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "timeout-watcher",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Spec: khapi.KuberhealthyCheckSpec{
			Timeout: &metav1.Duration{Duration: timeout},
		},
		Status: khapi.KuberhealthyCheckStatus{
			LastRunUnix: start.Unix(),
			CurrentUUID: "run-1",
			OK:          true,
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)
	kh.Recorder = record.NewFakeRecorder(1)

	kh.startTimeoutWatcher(check.DeepCopy())

	time.Sleep(timeout + timeoutGracePeriod + 300*time.Millisecond)

	updated := &khapi.KuberhealthyCheck{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "timeout-watcher"}, updated))
	require.False(t, updated.Status.OK)
	require.Len(t, updated.Status.Errors, 1)
	require.Contains(t, updated.Status.Errors[0], "timed out")
	require.Empty(t, updated.CurrentUUID())

	events := waitForEvents(t, kh.Recorder.(*record.FakeRecorder).Events, 1)
	require.Contains(t, events[0], "CheckRunTimedOut")
}

// TestTimeoutWatcherSkipsCompletedRun ensures the watcher exits quietly when the run finishes before the deadline.
func TestTimeoutWatcherSkipsCompletedRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout watcher completion test in short mode")
	}

	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	timeout := 500 * time.Millisecond
	start := time.Now()

	check := &khapi.KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "timeout-complete",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Spec: khapi.KuberhealthyCheckSpec{
			Timeout: &metav1.Duration{Duration: timeout},
		},
		Status: khapi.KuberhealthyCheckStatus{
			LastRunUnix: start.Unix(),
			CurrentUUID: "run-2",
			OK:          true,
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)
	kh.Recorder = record.NewFakeRecorder(1)

	kh.startTimeoutWatcher(check.DeepCopy())

	// simulate a successful report by clearing the UUID before the timeout expires
	completed := check.DeepCopy()
	completed.Status.CurrentUUID = ""
	require.NoError(t, cl.Status().Update(context.Background(), completed))

	time.Sleep(timeout + timeoutGracePeriod + 300*time.Millisecond)

	updated := &khapi.KuberhealthyCheck{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "timeout-complete"}, updated))
	require.True(t, updated.Status.OK)
	require.Empty(t, updated.Status.Errors)
	require.Empty(t, updated.CurrentUUID())

	select {
	case e := <-kh.Recorder.(*record.FakeRecorder).Events:
		t.Fatalf("unexpected event emitted: %s", e)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestScheduleSkipsWhenNotDue ensures scheduleChecks leaves checks untouched if their interval has not elapsed.
func TestScheduleSkipsWhenNotDue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping schedule skip test in short mode")
	}
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	last := time.Now().Unix()
	check := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kuberhealthy.github.io/v2",
		"kind":       "KuberhealthyCheck",
		"metadata": map[string]interface{}{
			"name":            "skip-check",
			"namespace":       "default",
			"resourceVersion": "1",
		},
		"spec": map[string]interface{}{
			"runInterval": "1h",
			"podSpec": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "test", "image": "busybox"},
					},
				},
			},
		},
		"status": map[string]interface{}{
			"lastRunUnix": last,
		},
	}}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)

	kh.scheduleChecks()

	fetched := &khapi.KuberhealthyCheck{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "skip-check", Namespace: "default"}, fetched))
	require.Empty(t, fetched.CurrentUUID())
	require.Equal(t, last, fetched.Status.LastRunUnix)
}

// TestScheduleLoopStopsOnStop verifies that Stop halts the scheduling loop.
func TestScheduleLoopStopsOnStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping schedule loop stop test in short mode")
	}
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	kh := New(context.Background(), cl)
	require.NoError(t, kh.Start(context.Background(), nil))
	// ensure loop has started
	time.Sleep(50 * time.Millisecond)

	kh.Stop()
	// give loop time to exit
	time.Sleep(50 * time.Millisecond)

	kh.loopMu.Lock()
	running := kh.loopRunning
	kh.loopMu.Unlock()
	require.False(t, running)
}
