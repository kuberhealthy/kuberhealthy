package kuberhealthy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newResponseTimeProbe builds an HTTP check with a maximum response time for the tests below.
func newResponseTimeProbe(url string, maxTime time.Duration) *khapi.HTTPCheck {
	return &khapi.HTTPCheck{URL: url, MaxResponseTime: &metav1.Duration{Duration: maxTime}}
}

// TestCheckResponseTimeIgnoresUnsetMaximum checks that a probe without maxResponseTime never fails on speed.
func TestCheckResponseTimeIgnoresUnsetMaximum(t *testing.T) {
	t.Parallel()

	err := checkResponseTime(&khapi.HTTPCheck{URL: "http://example.test"}, time.Hour)
	require.NoError(t, err)
}

// TestCheckResponseTimeAcceptsFastResponse checks that a response within the maximum passes.
func TestCheckResponseTimeAcceptsFastResponse(t *testing.T) {
	t.Parallel()

	err := checkResponseTime(newResponseTimeProbe("http://example.test", time.Second), 200*time.Millisecond)
	require.NoError(t, err)
}

// TestCheckResponseTimeFailsSlowResponse checks that a response over the maximum fails with both times in the error.
func TestCheckResponseTimeFailsSlowResponse(t *testing.T) {
	t.Parallel()

	err := checkResponseTime(newResponseTimeProbe("http://example.test", time.Second), 1500*time.Millisecond)
	require.EqualError(t, err, "response took 1.5s, which is more than maxResponseTime 1s")
}

// TestCheckResponseTimeRejectsNegativeMaximum checks that a negative maximum fails instead of failing every response silently.
func TestCheckResponseTimeRejectsNegativeMaximum(t *testing.T) {
	t.Parallel()

	err := checkResponseTime(newResponseTimeProbe("http://example.test", -time.Second), time.Millisecond)
	require.EqualError(t, err, "maxResponseTime must not be negative, got -1s")
}

// TestRunHTTPProbeFailsWhenResponseIsSlow checks the probe end to end against a server that delays its headers.
func TestRunHTTPProbeFailsWhenResponseIsSlow(t *testing.T) {
	t.Parallel()

	// the server waits before it answers
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	err := runHTTPProbe(context.Background(), newResponseTimeProbe(srv.URL, 50*time.Millisecond))
	require.ErrorContains(t, err, "more than maxResponseTime 50ms")
}

// TestRunHTTPProbePassesWhenResponseIsFastEnough checks the probe end to end against a server that answers right away.
func TestRunHTTPProbePassesWhenResponseIsFastEnough(t *testing.T) {
	t.Parallel()
	srv := newProbeTestServer(t, http.StatusOK, "")

	err := runHTTPProbe(context.Background(), newResponseTimeProbe(srv.URL, 5*time.Second))
	require.NoError(t, err)
}
