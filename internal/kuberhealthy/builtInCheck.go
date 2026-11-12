package kuberhealthy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

// maxProbeBodyBytes limits how much of an HTTP response body is read when matching its content.
const maxProbeBodyBytes = 1 << 20

// startBuiltInCheck records the run start and runs the check's HTTP or TCP probe in a goroutine.
// It is the pod-less counterpart of the pod creation in StartCheck.
func (kh *Kuberhealthy) startBuiltInCheck(ctx context.Context, checkName types.NamespacedName, startTime time.Time) error {
	// persist the start time so timeout tracking and the UI see the run
	err := kh.setLastRunTime(checkName, startTime)
	if err != nil {
		kh.finishStartCheck(checkName)
		return fmt.Errorf("unable to set check start time: %w", err)
	}

	// refresh the check so we act on the UUID and spec that were just stored
	check, err := kh.readCheck(checkName)
	if err != nil {
		kh.finishStartCheck(checkName)
		return fmt.Errorf("failed to refresh check %s after start: %w", checkName.String(), err)
	}

	// note the run in the check's events like a pod start would be
	if kh.Recorder != nil {
		kh.Recorder.Eventf(check, corev1.EventTypeNormal, "ProbeStarted", "built-in probe started at %s", startTime.Format(time.RFC3339))
	}

	// run the probe in the background and keep the timeout watcher as a safety net
	timeout := kh.checkTimeoutDuration(check)
	go kh.runBuiltInCheck(ctx, check, timeout)
	kh.startTimeoutWatcher(ctx, check)
	return nil
}

// runBuiltInCheck runs the probe for one check run and stores the result on the check status.
func (kh *Kuberhealthy) runBuiltInCheck(ctx context.Context, check *khapi.HealthCheck, timeout time.Duration) {
	// bound the probe by the check timeout
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// run the probe and time it
	started := time.Now()
	probeErr := runProbe(probeCtx, &check.Spec)
	duration := time.Since(started)

	// skip the result when the controller is shutting down or lost leadership
	if ctx.Err() != nil {
		log.Infof("built-in check %s/%s stopped before it finished: %v", check.Namespace, check.Name, ctx.Err())
		return
	}

	// store the result the same way a checker pod report would be stored
	checkName := types.NamespacedName{Namespace: check.Namespace, Name: check.Name}
	err := kh.recordBuiltInResult(checkName, check.CurrentUUID(), probeErr, duration)
	if err != nil {
		log.Errorf("built-in check %s/%s: failed to store result: %v", check.Namespace, check.Name, err)
		return
	}

	// surface the result as an event
	kh.recordBuiltInEvent(check, probeErr)
}

// recordBuiltInResult writes a probe result to the check status and clears the run UUID.
// Results for a run that is no longer current are dropped.
func (kh *Kuberhealthy) recordBuiltInResult(checkName types.NamespacedName, uuid string, probeErr error, duration time.Duration) error {
	// release the start guard once the result is stored or dropped
	defer kh.finishStartCheck(checkName)

	// retry on conflict so a concurrent status write does not lose the result
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return kh.applyBuiltInResult(checkName, uuid, probeErr, duration)
	})
}

// applyBuiltInResult reads the check, applies one probe result to its status and updates it.
func (kh *Kuberhealthy) applyBuiltInResult(checkName types.NamespacedName, uuid string, probeErr error, duration time.Duration) error {
	// read the current state of the check
	check, err := kh.readCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	// drop the result if the run timed out or a newer run started
	if check.CurrentUUID() != uuid {
		return nil
	}

	// apply the result and clear the UUID so the next run can be scheduled
	applyProbeResult(&check.Status, probeErr, time.Now())
	check.Status.LastRunDuration = duration
	check.Status.Namespace = check.Namespace
	check.SetCurrentUUID("")

	// store the status
	return khapi.UpdateCheck(kh.controllerContext(), kh.CheckClient, check)
}

// applyProbeResult sets the OK flag, errors and counters on a status like a checker pod report does.
func applyProbeResult(status *khapi.HealthCheckStatus, probeErr error, now time.Time) {
	// a failed probe counts as a failed run
	if probeErr != nil {
		status.OK = false
		status.Errors = []string{probeErr.Error()}
		status.ConsecutiveFailures++
		status.FailureCount++
		status.LastFailureUnix = now.Unix()
		return
	}

	// a successful probe resets the failure streak
	status.OK = true
	status.Errors = nil
	status.ConsecutiveFailures = 0
	status.SuccessCount++
	status.LastOKUnix = now.Unix()
}

// recordBuiltInEvent emits an event with the probe result.
func (kh *Kuberhealthy) recordBuiltInEvent(check *khapi.HealthCheck, probeErr error) {
	// events are optional
	if kh.Recorder == nil {
		return
	}

	// report failures as warnings
	if probeErr != nil {
		kh.Recorder.Event(check, corev1.EventTypeWarning, "CheckReported", probeErr.Error())
		return
	}

	// report success
	kh.Recorder.Event(check, corev1.EventTypeNormal, "CheckReported", "check reported OK")
}

// runProbe runs the HTTP or TCP probe described by the spec.
func runProbe(ctx context.Context, spec *khapi.HealthCheckSpec) error {
	// a check can only describe one kind of probe
	if spec.HTTP != nil && spec.TCP != nil {
		return errors.New("only one of http or tcp can be set")
	}

	// run the HTTP probe
	if spec.HTTP != nil {
		return runHTTPProbe(ctx, spec.HTTP)
	}

	// run the TCP probe
	if spec.TCP != nil {
		return runTCPProbe(ctx, spec.TCP)
	}

	return errors.New("no http or tcp probe is set")
}

// runTCPProbe succeeds when a TCP connection to the address can be opened.
func runTCPProbe(ctx context.Context, probe *khapi.TCPCheck) error {
	// dial the address within the probe deadline
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", probe.Address)
	if err != nil {
		return fmt.Errorf("tcp connection to %s failed: %w", probe.Address, err)
	}

	// close the connection right away, reachability is all we check
	closeErr := conn.Close()
	if closeErr != nil {
		log.Debugf("tcp probe: closing connection to %s: %v", probe.Address, closeErr)
	}
	return nil
}

// runHTTPProbe sends the request and checks the status code and body.
func runHTTPProbe(ctx context.Context, probe *khapi.HTTPCheck) error {
	// compile the body regex first so a bad pattern is reported clearly
	var bodyRegex *regexp.Regexp
	if probe.BodyRegex != "" {
		compiled, err := regexp.Compile(probe.BodyRegex)
		if err != nil {
			return fmt.Errorf("invalid bodyRegex: %w", err)
		}
		bodyRegex = compiled
	}

	// build the request
	method := probeMethod(probe)
	req, err := http.NewRequestWithContext(ctx, method, probe.URL, nil)
	if err != nil {
		return fmt.Errorf("invalid http request: %w", err)
	}
	for k, v := range probe.Headers {
		req.Header.Set(k, v)
	}

	// send the request
	client := newProbeHTTPClient(probe.InsecureSkipVerify)
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s failed: %w", method, probe.URL, err)
	}
	defer resp.Body.Close()

	// check how long the served certificates stay valid
	err = checkCertificateValidity(probe, resp.TLS, time.Now())
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, probe.URL, err)
	}

	// check the status code
	if !statusCodeExpected(probe.ExpectedStatusCodes, resp.StatusCode) {
		return fmt.Errorf("%s %s returned %d, expected %s", method, probe.URL, resp.StatusCode, describeExpectedCodes(probe.ExpectedStatusCodes))
	}

	// skip reading the body when nothing needs to match it
	if probe.BodyContains == "" && bodyRegex == nil {
		return nil
	}

	// read the body and match it
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProbeBodyBytes))
	if err != nil {
		return fmt.Errorf("%s %s: failed to read body: %w", method, probe.URL, err)
	}
	return matchBody(string(body), probe.BodyContains, bodyRegex)
}

// probeMethod returns the configured HTTP method or GET.
func probeMethod(probe *khapi.HTTPCheck) string {
	// default to GET
	if probe.Method == "" {
		return http.MethodGet
	}
	return strings.ToUpper(probe.Method)
}

// newProbeHTTPClient builds an HTTP client for one probe run.
func newProbeHTTPClient(insecureSkipVerify bool) *http.Client {
	// start from the default transport so proxy settings still apply
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: insecureSkipVerify, // #nosec G402 -- opt-in per check
	}
	return &http.Client{Transport: transport}
}

// statusCodeExpected reports whether the status code counts as success.
func statusCodeExpected(expected []int, code int) bool {
	// any 2xx code is fine when nothing is configured
	if len(expected) == 0 {
		return code >= 200 && code < 300
	}
	return slices.Contains(expected, code)
}

// describeExpectedCodes renders the expected status codes for an error message.
func describeExpectedCodes(expected []int) string {
	// the default is any 2xx code
	if len(expected) == 0 {
		return "2xx"
	}

	// list the configured codes
	parts := make([]string, 0, len(expected))
	for _, code := range expected {
		parts = append(parts, fmt.Sprintf("%d", code))
	}
	return strings.Join(parts, " or ")
}

// matchBody checks the response body against the configured string and regex.
func matchBody(body string, contains string, bodyRegex *regexp.Regexp) error {
	// check the plain substring
	if contains != "" && !strings.Contains(body, contains) {
		return fmt.Errorf("body did not contain %q", contains)
	}

	// check the regex
	if bodyRegex != nil && !bodyRegex.MatchString(body) {
		return fmt.Errorf("body did not match %q", bodyRegex.String())
	}
	return nil
}

// checkCertificateValidity fails when a certificate the server sent expires within the configured minimum validity.
func checkCertificateValidity(probe *khapi.HTTPCheck, state *tls.ConnectionState, now time.Time) error {
	// nothing to check when no minimum validity is configured
	if probe.MinCertificateValidity == nil {
		return nil
	}

	// a minimum validity only makes sense for a TLS response
	if state == nil || len(state.PeerCertificates) == 0 {
		return errors.New("minCertificateValidity is set but the response was not served over TLS")
	}

	// the certificate that expires first decides how long the endpoint stays usable
	cert := earliestExpiringCertificate(state.PeerCertificates)
	remaining := cert.NotAfter.Sub(now)
	minValidity := probe.MinCertificateValidity.Duration
	if remaining >= minValidity {
		return nil
	}

	// an expired certificate only gets here with insecureSkipVerify
	if remaining <= 0 {
		return fmt.Errorf("certificate %q expired at %s", cert.Subject.String(), cert.NotAfter.UTC().Format(time.RFC3339))
	}

	// report which certificate is about to expire and when
	return fmt.Errorf("certificate %q expires at %s, in %s, which is less than minCertificateValidity %s",
		cert.Subject.String(), cert.NotAfter.UTC().Format(time.RFC3339), remaining.Round(time.Second), minValidity)
}

// earliestExpiringCertificate returns the certificate of the chain with the earliest expiry.
func earliestExpiringCertificate(certs []*x509.Certificate) *x509.Certificate {
	// start from the leaf and keep the one that expires first
	earliest := certs[0]
	for _, cert := range certs[1:] {
		if cert.NotAfter.Before(earliest.NotAfter) {
			earliest = cert
		}
	}
	return earliest
}
