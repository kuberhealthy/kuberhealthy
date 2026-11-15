package kuberhealthy

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newTLSTestAddress starts a TLS server and returns its host:port and certificate expiry.
func newTLSTestAddress(t *testing.T) (string, time.Time) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "https://"), srv.Certificate().NotAfter
}

// TestRunTCPProbeCompletesTLSHandshake checks that a TLS handshake passes when verification is skipped.
func TestRunTCPProbeCompletesTLSHandshake(t *testing.T) {
	t.Parallel()
	addr, _ := newTLSTestAddress(t)

	err := runTCPProbe(context.Background(), &khapi.TCPCheck{Address: addr, TLS: true, InsecureSkipVerify: true})
	require.NoError(t, err)
}

// TestRunTCPProbeVerifiesTLSCertificate checks that an untrusted certificate fails the handshake.
func TestRunTCPProbeVerifiesTLSCertificate(t *testing.T) {
	t.Parallel()
	addr, _ := newTLSTestAddress(t)

	// the httptest certificate is self-signed, so normal verification rejects it
	err := runTCPProbe(context.Background(), &khapi.TCPCheck{Address: addr, TLS: true})
	require.ErrorContains(t, err, "tls handshake with "+addr+" failed")
}

// TestRunTCPProbeChecksTLSCertificateValidity checks the expiry threshold on the handshake certificates.
func TestRunTCPProbeChecksTLSCertificateValidity(t *testing.T) {
	t.Parallel()
	addr, notAfter := newTLSTestAddress(t)

	// a minimum well below the remaining validity passes
	probe := &khapi.TCPCheck{
		Address:                addr,
		TLS:                    true,
		InsecureSkipVerify:     true,
		MinCertificateValidity: &metav1.Duration{Duration: time.Hour},
	}
	require.NoError(t, runTCPProbe(context.Background(), probe))

	// a minimum beyond the expiry fails
	probe.MinCertificateValidity = &metav1.Duration{Duration: time.Until(notAfter) + 24*time.Hour}
	err := runTCPProbe(context.Background(), probe)
	require.ErrorContains(t, err, "tls "+addr+": certificate ")
	require.ErrorContains(t, err, "which is less than minCertificateValidity")
}

// TestRunTCPProbeFailsTLSAgainstPlainListener checks that a port that doesn't speak TLS fails the handshake.
func TestRunTCPProbeFailsTLSAgainstPlainListener(t *testing.T) {
	t.Parallel()

	// accept connections and close them without a handshake
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go acceptAndClose(ln)
	addr := ln.Addr().String()

	err = runTCPProbe(context.Background(), &khapi.TCPCheck{Address: addr, TLS: true, InsecureSkipVerify: true})
	require.ErrorContains(t, err, "tls handshake with "+addr+" failed")
}

// TestRunTCPProbeRejectsTLSOptionsWithoutTLS checks that TLS options are not ignored silently.
func TestRunTCPProbeRejectsTLSOptionsWithoutTLS(t *testing.T) {
	t.Parallel()

	probes := []*khapi.TCPCheck{
		{Address: "127.0.0.1:1", ServerName: "db.example.com"},
		{Address: "127.0.0.1:1", InsecureSkipVerify: true},
		{Address: "127.0.0.1:1", MinCertificateValidity: &metav1.Duration{Duration: time.Hour}},
	}
	for _, probe := range probes {
		err := runTCPProbe(context.Background(), probe)
		require.EqualError(t, err, "serverName, insecureSkipVerify and minCertificateValidity need tls: true")
	}
}

// TestTCPCheckDeepCopyCopiesMinCertificateValidity checks that a deep copy doesn't share the duration.
func TestTCPCheckDeepCopyCopiesMinCertificateValidity(t *testing.T) {
	t.Parallel()

	in := &khapi.TCPCheck{Address: "db:5432", TLS: true, MinCertificateValidity: &metav1.Duration{Duration: time.Hour}}
	out := in.DeepCopy()
	out.MinCertificateValidity.Duration = time.Minute
	require.Equal(t, time.Hour, in.MinCertificateValidity.Duration)
}
