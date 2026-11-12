package kuberhealthy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// newProbeTestServer starts an HTTP server that answers with the given status code and body.
func newProbeTestServer(t *testing.T, code int, body string) *httptest.Server {
	t.Helper()

	// write the fixed response for every request
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// TestRunHTTPProbeAcceptsAny2xxByDefault checks that a 2xx response passes when no codes are configured.
func TestRunHTTPProbeAcceptsAny2xxByDefault(t *testing.T) {
	t.Parallel()
	srv := newProbeTestServer(t, http.StatusNoContent, "")

	err := runHTTPProbe(context.Background(), &khapi.HTTPCheck{URL: srv.URL})
	require.NoError(t, err)
}

// TestRunHTTPProbeRejectsUnexpectedStatus checks the error for a status code that is not expected.
func TestRunHTTPProbeRejectsUnexpectedStatus(t *testing.T) {
	t.Parallel()
	srv := newProbeTestServer(t, http.StatusServiceUnavailable, "")

	err := runHTTPProbe(context.Background(), &khapi.HTTPCheck{URL: srv.URL, ExpectedStatusCodes: []int{200}})
	require.EqualError(t, err, "GET "+srv.URL+" returned 503, expected 200")
}

// TestRunHTTPProbeAcceptsConfiguredNon2xx checks that a configured non-2xx code passes.
func TestRunHTTPProbeAcceptsConfiguredNon2xx(t *testing.T) {
	t.Parallel()
	srv := newProbeTestServer(t, http.StatusUnauthorized, "")

	err := runHTTPProbe(context.Background(), &khapi.HTTPCheck{URL: srv.URL, ExpectedStatusCodes: []int{401}})
	require.NoError(t, err)
}

// TestRunHTTPProbeMatchesBody checks bodyContains and bodyRegex against the response body.
func TestRunHTTPProbeMatchesBody(t *testing.T) {
	t.Parallel()
	srv := newProbeTestServer(t, http.StatusOK, `{"status":"ok","version":"1.2.3"}`)

	err := runHTTPProbe(context.Background(), &khapi.HTTPCheck{URL: srv.URL, BodyContains: `"ok"`, BodyRegex: `version":"1\.\d+`})
	require.NoError(t, err)

	err = runHTTPProbe(context.Background(), &khapi.HTTPCheck{URL: srv.URL, BodyContains: "healthy"})
	require.EqualError(t, err, `body did not contain "healthy"`)

	err = runHTTPProbe(context.Background(), &khapi.HTTPCheck{URL: srv.URL, BodyRegex: `version":"2\.`})
	require.EqualError(t, err, `body did not match "version\":\"2\\."`)
}

// TestRunHTTPProbeRejectsInvalidRegex checks that a bad bodyRegex fails before any request is sent.
func TestRunHTTPProbeRejectsInvalidRegex(t *testing.T) {
	t.Parallel()

	err := runHTTPProbe(context.Background(), &khapi.HTTPCheck{URL: "http://127.0.0.1:1", BodyRegex: "("})
	require.ErrorContains(t, err, "invalid bodyRegex")
}

// TestRunHTTPProbeSendsMethodAndHeaders checks that the method and headers reach the server.
func TestRunHTTPProbeSendsMethodAndHeaders(t *testing.T) {
	t.Parallel()

	// record what the server received
	var gotMethod string
	var gotHeader string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeader = r.Header.Get("X-Probe")
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	probe := &khapi.HTTPCheck{URL: srv.URL, Method: "head", Headers: map[string]string{"X-Probe": "kuberhealthy"}}
	err := runHTTPProbe(context.Background(), probe)
	require.NoError(t, err)
	require.Equal(t, http.MethodHead, gotMethod)
	require.Equal(t, "kuberhealthy", gotHeader)
}

// TestRunTCPProbe checks that the TCP probe passes for an open port and fails for a closed one.
func TestRunTCPProbe(t *testing.T) {
	t.Parallel()

	// open a listener and accept connections in the background
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go acceptAndClose(ln)
	addr := ln.Addr().String()

	err = runTCPProbe(context.Background(), &khapi.TCPCheck{Address: addr})
	require.NoError(t, err)

	// close the listener so the port refuses connections
	require.NoError(t, ln.Close())
	err = runTCPProbe(context.Background(), &khapi.TCPCheck{Address: addr})
	require.ErrorContains(t, err, "tcp connection to "+addr+" failed")
}

// acceptAndClose accepts connections on the listener and closes them until the listener is closed.
func acceptAndClose(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
	}
}

// TestRunProbeRejectsBothKinds checks that a spec with both http and tcp fails.
func TestRunProbeRejectsBothKinds(t *testing.T) {
	t.Parallel()

	spec := &khapi.HealthCheckSpec{HTTP: &khapi.HTTPCheck{URL: "http://x"}, TCP: &khapi.TCPCheck{Address: "x:1"}}
	err := runProbe(context.Background(), spec)
	require.EqualError(t, err, "only one of http or tcp can be set")
}

// TestApplyProbeResult checks the status counters for a failure followed by a success.
func TestApplyProbeResult(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0)
	status := &khapi.HealthCheckStatus{}

	applyProbeResult(status, errors.New("boom"), now)
	require.False(t, status.OK)
	require.Equal(t, []string{"boom"}, status.Errors)
	require.Equal(t, 1, status.ConsecutiveFailures)
	require.Equal(t, 1, status.FailureCount)
	require.Equal(t, now.Unix(), status.LastFailureUnix)

	applyProbeResult(status, nil, now)
	require.True(t, status.OK)
	require.Empty(t, status.Errors)
	require.Equal(t, 0, status.ConsecutiveFailures)
	require.Equal(t, 1, status.SuccessCount)
	require.Equal(t, now.Unix(), status.LastOKUnix)
}

// newBuiltInTestCheck builds an HTTP check that points at the given URL.
func newBuiltInTestCheck(name string, url string) *khapi.HealthCheck {
	return &khapi.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: khapi.HealthCheckSpec{
			Timeout: &metav1.Duration{Duration: 5 * time.Second},
			HTTP:    &khapi.HTTPCheck{URL: url, BodyContains: "ok"},
		},
	}
}

// TestStartCheckBuiltInRecordsResultWithoutPod checks that an HTTP check runs in the controller,
// stores its result and creates no checker pod.
func TestStartCheckBuiltInRecordsResultWithoutPod(t *testing.T) {
	t.Parallel()
	srv := newProbeTestServer(t, http.StatusOK, "ok")

	// build a fake cluster with one HTTP check
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	check := newBuiltInTestCheck("http-check", srv.URL)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := newLeaderForStartCheckTest(cl)

	require.NoError(t, kh.StartCheck(check))

	// wait for the probe result to be stored
	nn := types.NamespacedName{Namespace: check.Namespace, Name: check.Name}
	require.Eventually(t, func() bool {
		stored, err := khapi.GetCheck(context.Background(), cl, nn)
		return err == nil && stored.Status.SuccessCount == 1
	}, 5*time.Second, 20*time.Millisecond)

	stored, err := khapi.GetCheck(context.Background(), cl, nn)
	require.NoError(t, err)
	require.True(t, stored.Status.OK)
	require.Empty(t, stored.Status.CurrentUUID)
	require.NotZero(t, stored.Status.LastRunUnix)

	// no checker pod should exist
	pods := &corev1.PodList{}
	require.NoError(t, cl.List(context.Background(), pods))
	require.Empty(t, pods.Items)

	// the start guard is released so the next run can start
	require.True(t, kh.tryStartCheck(nn))
}

// TestRecordBuiltInResultDropsStaleRun checks that a result for an old run UUID is not stored.
func TestRecordBuiltInResultDropsStaleRun(t *testing.T) {
	t.Parallel()

	// build a fake cluster with a check that is running a newer run
	scheme := runtime.NewScheme()
	require.NoError(t, khapi.AddToScheme(scheme))
	check := newBuiltInTestCheck("stale-check", "http://127.0.0.1:1")
	check.Status.CurrentUUID = "newer-run"
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
	kh := newLeaderForStartCheckTest(cl)
	nn := types.NamespacedName{Namespace: check.Namespace, Name: check.Name}

	require.NoError(t, kh.recordBuiltInResult(nn, "older-run", errors.New("boom"), time.Second))

	stored, err := khapi.GetCheck(context.Background(), cl, nn)
	require.NoError(t, err)
	require.Equal(t, "newer-run", stored.Status.CurrentUUID)
	require.Zero(t, stored.Status.FailureCount)
}
