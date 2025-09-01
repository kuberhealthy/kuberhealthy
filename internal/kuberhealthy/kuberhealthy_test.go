package kuberhealthy

import (
	"context"
	"strings"
	"testing"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := New(context.Background(), cl)

	pod := kh.CheckPodSpec(check)

	uuid, err := kh.getCurrentUUID(types.NamespacedName{Namespace: check.Namespace, Name: check.Name})
	require.NoError(t, err)

	require.Equal(t, check.Namespace, pod.Namespace)
	require.True(t, strings.HasPrefix(pod.Name, check.Name+"-"))
	require.Equal(t, check.Spec.PodSpec.Spec, pod.Spec)

	require.Equal(t, "kuberhealthy", pod.Annotations["createdBy"])
	require.Equal(t, check.Name, pod.Annotations["kuberhealthyCheckName"])
	require.NotEmpty(t, pod.Annotations["createdTime"])

	require.Equal(t, check.Name, pod.Labels[checkLabel])
	require.Equal(t, uuid, pod.Labels[runUUIDLabel])

	require.Len(t, pod.OwnerReferences, 1)
	owner := pod.OwnerReferences[0]
	require.Equal(t, check.Name, owner.Name)
	require.Equal(t, check.UID, owner.UID)
	require.Equal(t, khapi.GroupVersion.String(), owner.APIVersion)
	require.Equal(t, "KuberhealthyCheck", owner.Kind)
	require.NotNil(t, owner.Controller)
	require.True(t, *owner.Controller)
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

// TestScheduleStartsCheck confirms that scheduleChecks triggers a run when a check is due.
func TestScheduleStartsCheck(t *testing.T) {
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

// TestScheduleSkipsWhenNotDue ensures scheduleChecks leaves checks untouched if their interval has not elapsed.
func TestScheduleSkipsWhenNotDue(t *testing.T) {
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

// TestScheduleLoopOnlyRunsOnce checks that a second schedule loop invocation exits immediately if already running.
func TestScheduleLoopOnlyRunsOnce(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	kh := New(context.Background(), cl)
	require.NoError(t, kh.Start(context.Background(), nil))
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		kh.startScheduleLoop()
		close(done)
	}()

	select {
	case <-done:
		// second invocation exited immediately
	case <-time.After(200 * time.Millisecond):
		t.Fatal("second schedule loop did not exit")
	}
	kh.Stop()
}
