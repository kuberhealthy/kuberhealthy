package kuberhealthy

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// newProbeLifecycleController gives each test an isolated fake cluster and leader context.
func newProbeLifecycleController(t *testing.T, check *khapi.HealthCheck) (*Kuberhealthy, types.NamespacedName) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := newLeaderForStartCheckTest(cl)
	ctx, cancel := context.WithCancel(context.Background())
	kh.setLeaderContext(ctx, cancel)
	t.Cleanup(cancel)
	return kh, types.NamespacedName{Namespace: check.Namespace, Name: check.Name}
}

// newStalledProbeTransport simulates an HTTP peer with net.Pipe, without opening sockets.
func newStalledProbeTransport(t *testing.T) (<-chan struct{}, <-chan error) {
	t.Helper()
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	requested := make(chan struct{}, 1)
	closed := make(chan error, 1)
	transport := previous.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(context.Context, string, string) (net.Conn, error) {
		client, server := net.Pipe()
		t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
		go func() {
			_, err := http.ReadRequest(bufio.NewReader(server))
			if err != nil {
				return
			}
			requested <- struct{}{}
			var b [1]byte
			_, err = server.Read(b[:])
			closed <- err
		}()
		return client, nil
	}
	http.DefaultTransport = transport
	return requested, closed
}

// TestStopBuiltInCheckCancelsRequest verifies stopping and deleting cancel a stalled request.
// Serial because this test temporarily replaces the process-global http.DefaultTransport.
func TestStopBuiltInCheckCancelsRequest(t *testing.T) {
	for _, deletion := range []bool{false, true} {
		t.Run(fmt.Sprintf("delete=%t", deletion), func(t *testing.T) {
			requested, closed := newStalledProbeTransport(t)
			check := newBuiltInTestCheck("stalled", "http://probe.invalid")
			check.Spec.Timeout = &metav1.Duration{Duration: time.Hour}
			kh, nn := newProbeLifecycleController(t, check)
			require.NoError(t, kh.StartCheck(check))

			// Wait for a real in-process request before stopping its run.
			select {
			case <-requested:
			case <-time.After(time.Second):
				t.Fatal("probe did not send its request")
			}
			stored, err := kh.readCheck(nn)
			require.NoError(t, err)
			if deletion {
				kh.handleDelete(context.Background(), stored)
			}
			if !deletion {
				require.NoError(t, kh.StopCheck(stored))
			}

			// Cancellation must close the request, not merely clear its status UUID.
			select {
			case err = <-closed:
				require.Error(t, err)
			case <-time.After(time.Second):
				t.Fatal("stopped probe still holds its connection")
			}
			stored, err = kh.readCheck(nn)
			require.NoError(t, err)
			require.Empty(t, stored.CurrentUUID())
			require.Zero(t, stored.Status.SuccessCount)
			require.Zero(t, stored.Status.FailureCount)
		})
	}
}

// TestBuiltInResultRejectsExpiredSuccess preserves the pod report deadline for built-in results.
func TestBuiltInResultRejectsExpiredSuccess(t *testing.T) {
	t.Parallel()
	check := newBuiltInTestCheck("expired", "http://probe.invalid")
	check.Status.CurrentUUID = "expired-run"
	check.Status.LastRunUnix = time.Now().Add(-time.Hour).Unix()
	kh, nn := newProbeLifecycleController(t, check)
	recorder := record.NewFakeRecorder(4)
	kh.Recorder = recorder
	require.False(t, kh.IsReportAllowed(check, check.CurrentUUID()))

	// An expired success must leave the watcher responsible for recording the timeout.
	require.NoError(t, kh.recordBuiltInResult(nn, check.CurrentUUID(), nil, time.Millisecond))
	stored, err := kh.readCheck(nn)
	require.NoError(t, err)
	require.False(t, stored.Status.OK)
	require.Zero(t, stored.Status.SuccessCount)
	require.Equal(t, check.CurrentUUID(), stored.CurrentUUID())
	select {
	case event := <-recorder.Events:
		t.Fatalf("expired result emitted an event: %s", event)
	default:
	}

	kh.failRunIfOverdue(nn, check.CurrentUUID(), time.Unix(check.Status.LastRunUnix, 0), check.Spec.Timeout.Duration)
	stored, err = kh.readCheck(nn)
	require.NoError(t, err)
	require.Equal(t, 1, stored.Status.FailureCount)
	require.Empty(t, stored.CurrentUUID())
}

// TestBuiltInCheckUsesOriginalDeadline verifies bookkeeping cannot grant a fresh timeout.
// Serial because this test temporarily replaces the process-global http.DefaultTransport.
func TestBuiltInCheckUsesOriginalDeadline(t *testing.T) {
	requested, _ := newStalledProbeTransport(t)
	check := newBuiltInTestCheck("expired-start", "http://probe.invalid")
	check.Status.CurrentUUID = "expired-run"
	kh, nn := newProbeLifecycleController(t, check)
	require.True(t, kh.tryStartCheck(nn))

	// Model bookkeeping completing after the entire run deadline has passed.
	require.NoError(t, kh.startBuiltInCheck(kh.controllerContext(), nn, time.Now().Add(-time.Hour)))
	require.Eventually(t, func() bool {
		kh.builtInMu.Lock()
		defer kh.builtInMu.Unlock()
		return len(kh.builtInRuns) == 0
	}, time.Second, time.Millisecond)
	select {
	case <-requested:
		t.Fatal("probe sent a request after its original deadline")
	default:
	}
	require.Eventually(t, func() bool {
		stored, err := kh.readCheck(nn)
		return err == nil && stored.Status.FailureCount == 1 && stored.CurrentUUID() == ""
	}, time.Second, time.Millisecond)
	stored, err := kh.readCheck(nn)
	require.NoError(t, err)
	require.Zero(t, stored.Status.SuccessCount)
	require.False(t, stored.Status.OK)
}

// TestBuiltInCleanupPreservesNewerRun fences both cancellation and start-guard cleanup by UUID.
func TestBuiltInCleanupPreservesNewerRun(t *testing.T) {
	t.Parallel()
	check := newBuiltInTestCheck("replacement", "http://probe.invalid")
	check.Status.CurrentUUID = "new-run"
	kh, nn := newProbeLifecycleController(t, check)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	kh.builtInRuns = map[types.NamespacedName]builtInRun{
		nn: {uuid: "new-run", cancel: cancel},
	}
	require.True(t, kh.tryStartCheck(nn))

	// A stale stop or completion must neither cancel nor release the newer run.
	kh.builtInMu.Lock()
	kh.cancelBuiltInCheckLocked(nn, "old-run")
	kh.builtInMu.Unlock()
	kh.finishBuiltInCheck(nn, "old-run")
	require.NoError(t, ctx.Err())
	require.False(t, kh.tryStartCheck(nn))
	require.Len(t, kh.builtInRuns, 1)

	// Cancellation removes the entry but leaves guard release to StopCheck.
	kh.builtInMu.Lock()
	kh.cancelBuiltInCheckLocked(nn, "new-run")
	kh.builtInMu.Unlock()
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	kh.finishBuiltInCheck(nn, "new-run")
	require.Empty(t, kh.builtInRuns)
	require.False(t, kh.tryStartCheck(nn))
	require.NoError(t, kh.StopCheck(check))
	require.True(t, kh.tryStartCheck(nn))
}

// TestStopBuiltInCheckSerializesRegistration verifies cancellation and UUID clearing
// share the registration mutex, including when a stop precedes probe registration.
func TestStopBuiltInCheckSerializesRegistration(t *testing.T) {
	t.Parallel()
	check := newBuiltInTestCheck("stop-before-registration", "http://probe.invalid")
	check.Status.CurrentUUID = "stopped-run"
	kh, nn := newProbeLifecycleController(t, check)
	require.True(t, kh.tryStartCheck(nn))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	kh.builtInRuns = map[types.NamespacedName]builtInRun{
		nn: {uuid: check.CurrentUUID(), cancel: cancel},
	}

	// Observe the precise window that an unlocked cancel-then-clear would expose.
	cleared := false
	kh.CheckClient = interceptor.NewClient(kh.CheckClient.(client.WithWatch), interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, cl client.Client, name string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			current := obj.(*khapi.HealthCheck)
			if current.CurrentUUID() == "" && !cleared {
				locked := kh.builtInMu.TryLock()
				if locked {
					kh.builtInMu.Unlock()
				}
				require.False(t, locked, "registration can interleave with stopped UUID clearing")
				require.Empty(t, kh.builtInRuns)
				require.False(t, kh.tryStartCheck(nn), "guard released before stopped UUID clearing")
				cleared = true
			}
			return cl.SubResource(name).Update(ctx, obj, opts...)
		},
	})
	require.NoError(t, kh.StopCheck(check))
	require.True(t, cleared)
	require.ErrorIs(t, ctx.Err(), context.Canceled)

	// A delayed start that reaches registration after the stop must not launch a probe.
	require.NoError(t, kh.startBuiltInCheck(kh.controllerContext(), nn, time.Now()))
	require.Empty(t, kh.builtInRuns)
	stored, err := kh.readCheck(nn)
	require.NoError(t, err)
	require.Empty(t, stored.CurrentUUID())
}

// TestStoppedBuiltInCleanupPreservesUnregisteredReplacement reproduces completion
// arriving after a replacement acquired its guard but before registering its run.
func TestStoppedBuiltInCleanupPreservesUnregisteredReplacement(t *testing.T) {
	t.Parallel()
	for _, completedBeforeStop := range []bool{false, true} {
		t.Run(fmt.Sprintf("completedBeforeStop=%t", completedBeforeStop), func(t *testing.T) {
			t.Parallel()
			check := newBuiltInTestCheck("pre-registration", "http://probe.invalid")
			check.Status.CurrentUUID = "old-run"
			kh, nn := newProbeLifecycleController(t, check)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			kh.builtInRuns = map[types.NamespacedName]builtInRun{
				nn: {uuid: "old-run", cancel: cancel},
			}
			require.True(t, kh.tryStartCheck(nn))

			// Natural completion can win the cancellation mutex before StopCheck.
			if completedBeforeStop {
				kh.finishBuiltInCheck(nn, "old-run")
			}
			require.NoError(t, kh.StopCheck(check))
			require.ErrorIs(t, ctx.Err(), context.Canceled)

			// Pause the replacement precisely between guard acquisition and registration.
			require.True(t, kh.tryStartCheck(nn))
			kh.finishBuiltInCheck(nn, "old-run")
			require.False(t, kh.tryStartCheck(nn), "old completion released replacement guard")
			require.Empty(t, kh.builtInRuns)
		})
	}
}
