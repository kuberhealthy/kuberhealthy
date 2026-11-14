package kuberhealthy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newCertTestState builds a TLS connection state whose certificates expire at the given times.
func newCertTestState(notAfter ...time.Time) *tls.ConnectionState {
	// one certificate per expiry, the first one is the leaf
	certs := make([]*x509.Certificate, 0, len(notAfter))
	for i, expiry := range notAfter {
		cert := &x509.Certificate{
			Subject:  pkix.Name{CommonName: "cert-" + string(rune('a'+i))},
			NotAfter: expiry,
		}
		certs = append(certs, cert)
	}
	return &tls.ConnectionState{PeerCertificates: certs}
}

// newCertTestProbe builds an HTTP probe with the given minimum certificate validity.
func newCertTestProbe(minValidity time.Duration) *khapi.HTTPCheck {
	return &khapi.HTTPCheck{
		URL:                    "https://example.test",
		MinCertificateValidity: &metav1.Duration{Duration: minValidity},
	}
}

// TestCheckCertificateValiditySkippedWhenUnset checks that nothing is checked without a minimum validity.
func TestCheckCertificateValiditySkippedWhenUnset(t *testing.T) {
	t.Parallel()

	err := checkCertificateValidity(&khapi.HTTPCheck{}, nil, time.Now())
	require.NoError(t, err)
}

// TestCheckCertificateValidityRejectsNegativeMinimum checks that a negative minimum fails instead of passing every certificate.
func TestCheckCertificateValidityRejectsNegativeMinimum(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	state := newCertTestState(now.Add(-time.Hour))

	err := checkCertificateValidity(newCertTestProbe(-time.Hour), state, now)
	require.EqualError(t, err, "minCertificateValidity must not be negative, got -1h0m0s")
}

// TestCheckCertificateValidityRequiresTLS checks that a plain HTTP response fails when a minimum is set.
func TestCheckCertificateValidityRequiresTLS(t *testing.T) {
	t.Parallel()

	err := checkCertificateValidity(newCertTestProbe(time.Hour), nil, time.Now())
	require.EqualError(t, err, "minCertificateValidity is set but the response was not served over TLS")
}

// TestCheckCertificateValidityPassesWithEnoughTimeLeft checks that a certificate valid long enough passes.
func TestCheckCertificateValidityPassesWithEnoughTimeLeft(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	state := newCertTestState(now.Add(30*24*time.Hour), now.Add(365*24*time.Hour))

	err := checkCertificateValidity(newCertTestProbe(7*24*time.Hour), state, now)
	require.NoError(t, err)
}

// TestCheckCertificateValidityUsesEarliestCertificate checks that an intermediate expiring first fails the check.
func TestCheckCertificateValidityUsesEarliestCertificate(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	state := newCertTestState(now.Add(90*24*time.Hour), now.Add(48*time.Hour))

	err := checkCertificateValidity(newCertTestProbe(7*24*time.Hour), state, now)
	require.EqualError(t, err, `certificate "CN=cert-b" expires at 2026-01-03T00:00:00Z, in 48h0m0s, which is less than minCertificateValidity 168h0m0s`)
}

// TestCheckCertificateValidityReportsExpired checks the error for a certificate that already expired.
func TestCheckCertificateValidityReportsExpired(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	state := newCertTestState(now.Add(-time.Hour))

	err := checkCertificateValidity(newCertTestProbe(time.Hour), state, now)
	require.EqualError(t, err, `certificate "CN=cert-a" expired at 2025-12-31T23:00:00Z`)
}

// TestRunHTTPProbeChecksServedCertificate checks the minimum validity against a real TLS server.
func TestRunHTTPProbeChecksServedCertificate(t *testing.T) {
	t.Parallel()

	// the httptest certificate is self-signed, so skip verification and only look at its expiry
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	notAfter := srv.Certificate().NotAfter
	untilExpiry := time.Until(notAfter)

	// a minimum well below the remaining validity passes
	probe := newCertTestProbe(time.Hour)
	probe.URL = srv.URL
	probe.InsecureSkipVerify = true
	err := runHTTPProbe(context.Background(), probe)
	require.NoError(t, err)

	// a minimum beyond the expiry fails
	probe.MinCertificateValidity = &metav1.Duration{Duration: untilExpiry + 24*time.Hour}
	err = runHTTPProbe(context.Background(), probe)
	require.ErrorContains(t, err, "GET "+srv.URL+": certificate ")
	require.ErrorContains(t, err, "which is less than minCertificateValidity")
}

// TestRunHTTPProbeRejectsMinValidityOverPlainHTTP checks that a minimum validity fails over plain HTTP.
func TestRunHTTPProbeRejectsMinValidityOverPlainHTTP(t *testing.T) {
	t.Parallel()
	srv := newProbeTestServer(t, http.StatusOK, "")

	probe := newCertTestProbe(time.Hour)
	probe.URL = srv.URL
	err := runHTTPProbe(context.Background(), probe)
	require.EqualError(t, err, "GET "+srv.URL+": minCertificateValidity is set but the response was not served over TLS")
}
