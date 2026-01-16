package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	yaml "github.com/ghodss/yaml"
	"github.com/google/uuid"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/health"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/metrics"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultRunInterval   = time.Minute * 10
	defaultReportTimeout = 30 * time.Second // keep in sync with kuberhealthy.defaultRunTimeout
)

// reportAllowed determines if an incoming report should be accepted when the main controller is unavailable.
// Tests rely on this helper so that timeout enforcement remains active even without Globals.kh.
func reportAllowed(check *khapi.HealthCheck, uuid string) bool {
	if check == nil {
		return true
	}
	if uuid == "" {
		return false
	}

	if Globals.kh != nil {
		allowed := Globals.kh.IsReportAllowed(check, uuid)
		log.WithFields(log.Fields{
			"check":       types.NamespacedName{Name: check.Name, Namespace: check.Namespace},
			"reportUUID":  uuid,
			"currentUUID": check.CurrentUUID(),
			"lastRunUnix": check.Status.LastRunUnix,
			"allowed":     allowed,
		}).Debug("report allowance evaluated")
		return allowed
	}

	if check.CurrentUUID() != "" && check.CurrentUUID() != uuid {
		return false
	}

	timeout := defaultReportTimeout
	if check.Spec.Timeout != nil && check.Spec.Timeout.Duration > 0 {
		timeout = check.Spec.Timeout.Duration
	}
	if timeout <= 0 {
		return true
	}

	if check.Status.LastRunUnix == 0 {
		return true
	}

	started := time.Unix(check.Status.LastRunUnix, 0)
	if started.IsZero() {
		return true
	}

	if time.Since(started) >= timeout {
		return false
	}

	return true
}

// requestLogger logs incoming requests before they reach a handler.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// determine the source IP making the request
		ip := r.Header.Get("X-Forwarded-For")
		if ip != "" {
			parts := strings.Split(ip, ",")
			ip = strings.TrimSpace(parts[0])
		}
		if ip == "" {
			// fall back to parsing the remote address when no forwarded header is present
			host, _, splitErr := net.SplitHostPort(r.RemoteAddr)
			if splitErr == nil {
				ip = host
			}
			if splitErr != nil {
				ip = r.RemoteAddr
			}
		}

		// capture the user agent string
		ua := r.UserAgent()

		// log the IP, user agent, HTTP method, and path
		log.WithFields(log.Fields{
			"ip":     ip,
			"ua":     ua,
			"method": r.Method,
			"path":   r.URL.Path,
		}).Info("Client request")

		// continue handling the request
		next.ServeHTTP(w, r)
	})
}

// healthCheckParam returns the requested healthcheck name from the query string.
func healthCheckParam(r *http.Request) string {
	value := strings.TrimSpace(r.URL.Query().Get("healthcheck"))
	return value
}

// podBelongsToHealthCheck reports whether the supplied pod is labeled for the provided healthcheck name.
// It accepts the `healthcheck` label that Kuberhealthy applies to checker pods.
func podBelongsToHealthCheck(pod *v1.Pod, healthCheck string) bool {
	if pod == nil {
		return false
	}
	if healthCheck == "" {
		return false
	}
	if pod.Labels == nil {
		return false
	}
	return pod.Labels["healthcheck"] == healthCheck
}

// newServeMux configures and returns a mux with all web handlers mounted.
func newServeMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Metrics endpoint for prometheus metrics
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		err := prometheusMetricsHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// General healthcheck endpoint for heartbeats
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		err := healthzHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// JSON status page for easy API integration
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		err := healthCheckHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// UI Endpoint for listing healthcheck events
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		err := eventListHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// UI Endpoint for fetching healthcheck pod logs
	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		err := podLogsHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// UI Endpoint for tailing healthcheck pod logs
	mux.HandleFunc("/api/logs/stream", func(w http.ResponseWriter, r *http.Request) {
		err := podLogsStreamHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// YAML output of OpenAPI spec
	mux.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		err := openapiYAMLHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// JSON output of OpenAPI spec
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		err := openapiJSONHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// static asset hosting for web ui
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./assets"))))

	// allows the web interface to kick off a healthcheck run immediately
	mux.HandleFunc("/api/run", func(w http.ResponseWriter, r *http.Request) {
		err := runCheckHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// this is where healthcheck pods report back with their check status
	mux.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		err := checkReportHandler(w, r)
		if err != nil {
			log.Errorln("checkStatus endpoint error:", err)
		}
	})

	// This is the main handler that serves the web interface.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// only accept GET for the UI to keep report handling on /check
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// serve the UI for GET requests
		err := statusPageHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	return mux
}

// startTLSServer launches the HTTPS listener when certificates are provided.
func startTLSServer(certFile string, keyFile string, handler http.Handler) {
	log.Infoln("TLS ceritificate and key configured. Starting TLS listener on", GlobalConfig.ListenAddressTLS)

	// load cert and key
	_, certErr := os.Stat(GlobalConfig.TLSCertFile)
	_, keyErr := os.Stat(GlobalConfig.TLSKeyFile)

	// if cert failed to load, throw error
	if certErr != nil {
		log.Errorf("failed to start secure web server with cert %s and key %s due to error %v", GlobalConfig.TLSCertFile, GlobalConfig.TLSKeyFile, certErr)
	}

	// if key failed to load the key, but did load the cert, throw error
	if certErr == nil && keyErr != nil {
		log.Errorf("failed to start secure web server with cert %s and key %s due to error %v", GlobalConfig.TLSCertFile, GlobalConfig.TLSKeyFile, keyErr)
	}

	// if there aren't cert loading errors, start the TLS web server in a go routine and throw an error if it fails to start up
	if certErr == nil && keyErr == nil {
		go func() {
			err := http.ListenAndServeTLS(GlobalConfig.ListenAddressTLS, GlobalConfig.TLSCertFile, GlobalConfig.TLSKeyFile, handler)
			if err != nil {
				log.Errorln("TLS listener failed to setup with error:", err)
			}
		}()
	}

}

// startHTTPServer launches the HTTP listener for the public endpoints.
func startHTTPServer(handler http.Handler) {
	log.Infoln("Starting HTTP web services on", GlobalConfig.ListenAddress)
	go func() {
		err := http.ListenAndServe(GlobalConfig.ListenAddress, handler)
		if err != nil {
			log.Errorln("HTTP listener failed to setup with error:", err)
		}
	}()
}

// StartWebServer starts the web server with all handlers attached to the muxer.
func StartWebServer() error {

	// build a muxer to route requests, then wrap it so it always writes logs
	mux := newServeMux()
	handler := requestLogger(mux)

	// listen on normal http always
	startHTTPServer(handler)

	// if tls cert and key is set, then listen using TLS
	if GlobalConfig.TLSCertFile != "" && GlobalConfig.TLSKeyFile != "" {
		startTLSServer(GlobalConfig.TLSCertFile, GlobalConfig.TLSKeyFile, handler)
	}

	return nil
}

// renderOpenAPISpec writes the OpenAPI spec as JSON.
func renderOpenAPISpec(w http.ResponseWriter) error {
	data, err := os.ReadFile("./openapi.yaml")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = w.Write(jsonData)
	return err
}

// openapiYAMLHandler serves the OpenAPI spec at /openapi.yaml as JSON.
func openapiYAMLHandler(w http.ResponseWriter, _ *http.Request) error {
	return renderOpenAPISpec(w)
}

// openapiJSONHandler serves the OpenAPI spec at /openapi.json as JSON.
func openapiJSONHandler(w http.ResponseWriter, _ *http.Request) error {
	return renderOpenAPISpec(w)
}

// runCheckHandler triggers an immediate run of a check.
func runCheckHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	healthCheck := healthCheckParam(r)
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if healthCheck == "" || namespace == "" {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("missing parameters")
	}
	if Globals.khClient == nil || Globals.kh == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kuberhealthy not initialized")
	}
	if !Globals.kh.IsLeader() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kuberhealthy instance is not leader")
	}
	nn := types.NamespacedName{Namespace: namespace, Name: healthCheck}
	check, err := khapi.GetCheck(r.Context(), Globals.khClient, nn)
	if err != nil {
		if apierrors.IsNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return err
	}
	if check.CurrentUUID() != "" {
		w.WriteHeader(http.StatusConflict)
		return fmt.Errorf("check already running")
	}
	err = Globals.kh.StartCheck(check)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	// reset the check ticker so the next automatic run starts from now
	check.Status.LastRunUnix = time.Now().Unix()
	err = khapi.UpdateCheck(r.Context(), Globals.khClient, check)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("failed to update last run time: %w", err)
	}
	w.WriteHeader(http.StatusAccepted)
	return nil
}

type eventSummary struct {
	Message       string `json:"message"`
	Reason        string `json:"reason"`
	Type          string `json:"type"`
	LastTimestamp int64  `json:"lastTimestamp,omitempty"`
}

// eventListHandler returns events for a given healthcheck.
func eventListHandler(w http.ResponseWriter, r *http.Request) error {
	healthCheck := healthCheckParam(r)
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if healthCheck == "" || namespace == "" {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("missing parameters")
	}
	if Globals.kubeClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	fs := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=HealthCheck", healthCheck)
	evList, err := Globals.kubeClient.CoreV1().Events(namespace).List(r.Context(), metav1.ListOptions{FieldSelector: fs})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	summaries := make([]eventSummary, 0, len(evList.Items))
	for _, e := range evList.Items {
		s := eventSummary{Message: e.Message, Reason: e.Reason, Type: e.Type}
		if !e.LastTimestamp.IsZero() {
			s.LastTimestamp = e.LastTimestamp.Unix()
		}
		summaries = append(summaries, s)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(summaries)
}

// podLogsHandler returns logs and details for a specific pod.
func podLogsHandler(w http.ResponseWriter, r *http.Request) error {
	podName := strings.TrimSpace(r.URL.Query().Get("pod"))
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	healthCheck := healthCheckParam(r)
	if podName == "" || namespace == "" || healthCheck == "" {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("missing parameters")
	}
	if Globals.khClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	pod := &v1.Pod{}
	err := Globals.khClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: podName}, pod)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if !podBelongsToHealthCheck(pod, healthCheck) {
		w.WriteHeader(http.StatusForbidden)
		return fmt.Errorf("pod not part of healthcheck")
	}
	if Globals.kubeClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	req := Globals.kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{})
	stream, err := req.Stream(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	defer stream.Close()
	b, err := io.ReadAll(stream)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if r.URL.Query().Get("format") != "text" {
		w.WriteHeader(http.StatusNotAcceptable)
		return fmt.Errorf("format must be 'text'")
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, writeErr := w.Write(b)
	if writeErr != nil {
		return writeErr
	}
	return nil
}

// podLogsStreamHandler streams logs for a specific pod in real time.
func podLogsStreamHandler(w http.ResponseWriter, r *http.Request) error {
	podName := strings.TrimSpace(r.URL.Query().Get("pod"))
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	healthCheck := healthCheckParam(r)
	if podName == "" || namespace == "" || healthCheck == "" {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("missing parameters")
	}
	if Globals.khClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	pod := &v1.Pod{}
	err := Globals.khClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: podName}, pod)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if !podBelongsToHealthCheck(pod, healthCheck) {
		w.WriteHeader(http.StatusForbidden)
		return fmt.Errorf("pod not part of healthcheck")
	}
	if pod.Status.Phase != v1.PodRunning {
		w.WriteHeader(http.StatusConflict)
		return fmt.Errorf("pod not running")
	}
	nn := types.NamespacedName{Namespace: namespace, Name: healthCheck}
	_, err = khapi.GetCheck(r.Context(), Globals.khClient, nn)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return err
	}
	if Globals.kubeClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	req := Globals.kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{Follow: true})
	stream, err := req.Stream(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	defer stream.Close()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("streaming not supported")
	}
	buf := make([]byte, 1024)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			flusher.Flush()
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// PodReportInfo holds info about an incoming IP to the external check reporting endpoint
type PodReportInfo struct {
	Name      string
	UUID      string
	Namespace string
}

// function variables allow tests to stub dependencies
var (
	validateUsingRequestHeaderFunc = validateUsingRequestHeader
	storeCheckStateFunc            = storeCheckState
)

// prometheusMetricsHandler is a handler for all prometheus metrics requests
func prometheusMetricsHandler(w http.ResponseWriter, _ *http.Request) error {
	state := getCurrentState([]string{})

	m := metrics.GenerateMetrics(state, GlobalConfig.PromMetricsConfig)
	// write summarized health check results back to caller
	_, err := w.Write([]byte(m))
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}

// healthzHandler performs basic checks and writes OK when Kuberhealthy is healthy.
func healthzHandler(w http.ResponseWriter, _ *http.Request) error {
	if Globals.kubeClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	_, err := Globals.kubeClient.Discovery().ServerVersion()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes API unavailable: %w", err)
	}
	w.WriteHeader(http.StatusOK)
	_, err = io.WriteString(w, "OK")
	return err
}

// healthCheckHandler returns the current status of checks loaded into Kuberhealthy
// as JSON to the client. Respects namespace requests via URL query parameters (i.e. /?namespace=default)
func healthCheckHandler(w http.ResponseWriter, r *http.Request) error {
	// log.Infoln("Client connected to status page from", r.RemoteAddr, r.UserAgent())

	// If a request body was supplied, throw an error to ensure that checks don't report into the wrong url
	body, err := io.ReadAll(r.Body)
	if len(body) > 0 {
		log.Warningln("Unexpected body from status page request. Verify check is reporting to the right status url", r.RemoteAddr, r.RequestURI)
		w.WriteHeader(http.StatusBadRequest)
		return err
	}

	// get URL query parameters if there are any
	values := r.URL.Query()
	namespaceValue := values.Get("namespace")
	// .Get() will return an "" if there is no value associated -- we do not want to pass "" as a requested namespace
	var namespaces []string
	if len(namespaceValue) != 0 {
		namespaceSplits := strings.Split(namespaceValue, ",")
		// a query like (/?namespace=,) will cause .Split() to return an array of two empty strings ["", ""]
		// so we need to filter those out
		for _, namespaceSplit := range namespaceSplits {
			if len(namespaceSplit) != 0 {
				namespaces = append(namespaces, namespaceSplit)
			}
		}
	}

	// fetch the current status from our healthcheck resources
	state := getCurrentState(namespaces)

	// write summarized health check results back to caller
	err = state.WriteHTTPStatusResponse(w)
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}

// checkReportHandler handles requests coming from external checkers reporting their status.
// This endpoint checks that the external check report is coming from the correct UUID or pod IP before recording
// the reported status of the corresponding external check.  This endpoint expects a JSON payload of
// the `State` struct found in the github.com/kuberhealthy/kuberhealthy/v2/pkg/health package.  The request
// causes a check of the calling pod's spec via the API to ensure that the calling pod is expected
// to be reporting its status.
func checkReportHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return nil
	}

	// make a request ID for tracking this request
	requestID := "web: " + uuid.New().String()

	ctx := r.Context()

	// log.Println("webserver:", requestID, "Client connected to check report handler from", r.UserAgent())

	// Validate request using the kh-run-uuid header.
	log.Println("webserver:", requestID, "validating external check status report from run uuid:", r.Header.Get("kh-run-uuid"))
	podReport, reportValidated, err := validateUsingRequestHeaderFunc(ctx, r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		log.Println("webserver:", requestID, "Failed to look up healthcheck by its kh-run-uuid header:", r.Header.Get("kh-run-uuid"), err)
		return nil
	}
	if !reportValidated {
		w.WriteHeader(http.StatusBadRequest)
		log.Println("webserver:", requestID, "Request missing kh-run-uuid header")
		return nil
	}
	log.Println("webserver:", requestID, "Reporting check is", podReport.Name, "in namespace", podReport.Namespace)

	// append pod info to request id for easy check tracing in logs
	requestID = requestID + " (" + podReport.Namespace + "/" + podReport.Name + ")"

	// fetch the healthcheck for event recording when a client is available
	var healthCheck *khapi.HealthCheck
	clientForCheck := Globals.khClient
	if clientForCheck == nil && Globals.kh != nil {
		clientForCheck = Globals.kh.CheckClient
	}
	if clientForCheck != nil {
		nn := types.NamespacedName{Name: podReport.Name, Namespace: podReport.Namespace}
		var err error
		healthCheck, err = khapi.GetCheck(ctx, clientForCheck, nn)
		if err != nil {
			log.Println("webserver:", requestID, "failed to fetch healthcheck for event recording:", err)
			healthCheck = nil
		}
	}

	if healthCheck != nil {
		log.WithFields(log.Fields{
			"check":       types.NamespacedName{Name: healthCheck.Name, Namespace: healthCheck.Namespace},
			"currentUUID": healthCheck.CurrentUUID(),
			"reportUUID":  podReport.UUID,
			"lastRunUnix": healthCheck.Status.LastRunUnix,
		}).Debug("report gating snapshot")
	}
	if !reportAllowed(healthCheck, podReport.UUID) {
		w.WriteHeader(http.StatusGone)
		log.Println("webserver:", requestID, "Rejected report after timeout for", podReport.Namespace, podReport.Name)
		return nil
	}

	// ensure the client is sending a valid payload in the request body
	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Println("webserver:", requestID, "Failed to read request body:", err.Error(), r.RemoteAddr)
		return nil
	}
	log.Debugln("Check report body:", string(b))

	// decode the bytes into a status struct as used by the client
	state := health.Report{}
	err = json.Unmarshal(b, &state)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Println("webserver:", requestID, "Failed to unmarshal state json:", err, r.RemoteAddr)
		return nil
	}
	log.Debugf("Check report after unmarshal: +%v\n", state)

	// ensure that reports do not contain errors when OK is true
	if state.OK && len(state.Errors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		log.Println("webserver:", requestID, "Client attempted to report OK true with error strings")
		return nil
	}

	// ensure that if ok is set to false, then an error is provided
	if !state.OK {
		if len(state.Errors) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			log.Println("webserver:", requestID, "Client attempted to report OK false without any error strings")
			return nil
		}
		for _, e := range state.Errors {
			if len(e) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				log.Println("webserver:", requestID, "Client attempted to report a blank error string")
				return nil
			}
		}
	}

	// checkRunDuration := time.Duration(0).String()

	// checkDetails := stateReflector.CurrentStatus().CheckDetails
	// checkRunDuration = checkDetails[podReport.Namespace+"/"+podReport.Name].RunDuration

	// create a details object from our incoming status report before storing it on the healthcheck status field
	details := &khapi.HealthCheckStatus{}
	details.Errors = state.Errors
	details.OK = state.OK
	// details.RunDuration = checkRunDuration
	details.Namespace = podReport.Namespace
	// clear the UUID so that the scheduler can start the next run
	details.CurrentUUID = ""
	// since the check is validated, we can proceed to update the status now
	log.Println("webserver:", requestID, "Setting check with name", podReport.Name, "in namespace", podReport.Namespace, "to 'OK' state:", details.OK, "uuid", details.CurrentUUID)
	err = storeCheckStateFunc(Globals.khClient, podReport.Name, podReport.Namespace, details)
	if err != nil {
		if healthCheck != nil && Globals.kh != nil && Globals.kh.Recorder != nil {
			Globals.kh.Recorder.Eventf(healthCheck, v1.EventTypeWarning, "CheckReportFailed", "failed to store check state: %v", err)
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Println("webserver:", requestID, "failed to store check state for %s: %w", podReport.Name, err)
		return fmt.Errorf("failed to store check state for %s: %w", podReport.Name, err)
	}
	log.WithFields(log.Fields{
		"namespace": podReport.Namespace,
		"name":      podReport.Name,
		"ok":        state.OK,
		"errors":    state.Errors,
	}).Info("checker pod reported")

	if healthCheck != nil && Globals.kh != nil && Globals.kh.Recorder != nil {
		if state.OK {
			Globals.kh.Recorder.Event(healthCheck, v1.EventTypeNormal, "CheckReported", "check reported OK")
		} else {
			Globals.kh.Recorder.Eventf(healthCheck, v1.EventTypeWarning, "CheckReported", strings.Join(state.Errors, "; "))
		}
	}

	// write ok back to caller
	w.WriteHeader(http.StatusOK)
	log.Println("webserver:", requestID, "Request completed successfully.")
	return nil
}

// getCurrentState fetches the current state of all checks from requested namespaces
// their CRD objects and returns the summary as a health.State. Without a requested namespace,
// this will return the state of ALL found checks.
// Failures to fetch CRD state return an error.
func getCurrentState(namespaces []string) health.State {
	return getCurrentStatusForNamespaces(namespaces)
}

// getCurrentState fetches the current state of all checks from the requested namespaces
// their CRD objects and returns the summary as a health.State.
// Failures to fetch CRD state return an error.
func getCurrentStatusForNamespaces(namespaces []string) health.State {
	state := health.NewState()
	if Globals.khClient == nil {
		state.OK = false
		state.AddError("kubernetes client not initialized")
		return state
	}

	ctx := context.Background()
	uList := &unstructured.UnstructuredList{}
	uList.SetGroupVersionKind(khapi.GroupVersion.WithKind("HealthCheckList"))
	var opts []client.ListOption
	if len(namespaces) == 1 {
		opts = append(opts, client.InNamespace(namespaces[0]))
	}
	err := Globals.khClient.List(ctx, uList, opts...)
	if err != nil {
		state.OK = false
		state.AddError(err.Error())
		return state
	}

	for _, u := range uList.Items {
		if len(namespaces) > 1 && !containsString(u.GetNamespace(), namespaces) {
			continue
		}

		runInterval := runIntervalForCheck(&u)
		timeout := timeoutForCheck(&u)

		var check khapi.HealthCheck
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &check)
		if err != nil {
			log.Errorf("failed to convert check %s/%s: %v", u.GetNamespace(), u.GetName(), err)
			continue
		}
		check.EnsureCreationTimestamp()

		status := health.CheckDetail{HealthCheckStatus: check.Status}
		status.RunIntervalSeconds = int64(runInterval.Seconds())
		if timeout > 0 {
			status.TimeoutSeconds = int64(timeout.Seconds())
		}
		if status.Namespace == "" {
			status.Namespace = check.Namespace
		}
		if check.CurrentUUID() != "" {
			pods := &v1.PodList{}
			err := Globals.khClient.List(ctx, pods,
				client.InNamespace(check.Namespace),
				client.MatchingLabels{"kh-run-uuid": check.CurrentUUID()},
			)
			if err == nil && len(pods.Items) > 0 {
				status.PodName = pods.Items[0].Name
			}
		} else if check.Status.LastRunUnix != 0 {
			next := time.Unix(check.Status.LastRunUnix, 0).Add(runInterval)
			status.NextRunUnix = next.Unix()
		}
		state.CheckDetails[check.Name] = status
		if !status.OK {
			state.OK = false
			state.AddError(status.Errors...)
		}
	}

	return state
}

// runIntervalForCheck extracts the configured run interval for a check object.
func runIntervalForCheck(u *unstructured.Unstructured) time.Duration {
	if v, found, err := unstructured.NestedString(u.Object, "spec", "runInterval"); err == nil && found && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Errorf("invalid runInterval %q on check %s/%s", v, u.GetNamespace(), u.GetName())
	}
	return defaultRunInterval
}

// timeoutForCheck extracts the timeout duration from a check object when present.
func timeoutForCheck(u *unstructured.Unstructured) time.Duration {
	if v, found, err := unstructured.NestedString(u.Object, "spec", "timeout"); err == nil && found && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Errorf("invalid timeout %q on check %s/%s", v, u.GetNamespace(), u.GetName())
	}
	return 0
}

// validateUsingRequestHeader ensures the request includes a valid kh-run-uuid header and
// returns information about the associated HealthCheck.
func validateUsingRequestHeader(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {
	var info PodReportInfo
	uuid := r.Header.Get("kh-run-uuid")
	if uuid == "" {
		return info, false, nil
	}
	check, err := findCheckByUUID(ctx, uuid)
	if err != nil {
		return info, true, fmt.Errorf("failed to lookup healthcheck for uuid %s: %w", uuid, err)
	}
	if check == nil {
		return info, true, fmt.Errorf("run uuid %s is invalid or expired", uuid)
	}
	info.Name = check.Name
	info.Namespace = check.Namespace
	info.UUID = uuid
	return info, true, nil
}

// findCheckByUUID searches for a HealthCheck whose status CurrentUUID matches the provided uuid.
func findCheckByUUID(ctx context.Context, uuid string) (*khapi.HealthCheck, error) {
	if Globals.khClient == nil {
		return nil, fmt.Errorf("kubernetes client not initialized")
	}
	list := &khapi.HealthCheckList{}
	err := Globals.khClient.List(ctx, list)
	if err != nil {
		return nil, err
	}
	for i := range list.Items {
		if list.Items[i].CurrentUUID() == uuid {
			return &list.Items[i], nil
		}
	}
	return nil, nil
}

// storeCheckState stores the healthcheck status on its cluster CRD.
func storeCheckState(c client.Client, checkName string, checkNamespace string, healthCheckStatus *khapi.HealthCheckStatus) error {
	// ensure the CRD resource exists
	err := ensureCheckResourceExists(c, checkName, checkNamespace)
	if err != nil {
		return err
	}

	// put the status on the CRD from the check
	err = setCheckStatus(c, checkName, checkNamespace, healthCheckStatus)

	//TODO: Make this retry of updating custom resources repeatable
	//
	// We commonly see a race here with the following type of error:
	// "Error storing CRD state for check: pod-restarts in namespace kuberhealthy Operation cannot be fulfilled on healthchecks.kuberhealthy.github.io \"pod-restarts\": the object
	// has been modified; please apply your changes to the latest version and try again"
	//
	// If we see this error, we fetch the updated object, re-apply our changes, and try again
	delay := time.Duration(time.Second * 1)
	maxTries := 7
	tries := 0
	for err != nil && strings.Contains(err.Error(), "the object has been modified") {
		// if too many retries have occurred, we fail up the stack further
		if tries > maxTries {
			return fmt.Errorf("failed to update healthcheck status for check %s in namespace %s after %d with error %w", checkName, checkNamespace, maxTries, err)
		}
		log.Warnln("Failed to update healthcheck status because object was modified by another process.  Retrying in " + delay.String() + ".  Try " + strconv.Itoa(tries) + " of " + strconv.Itoa(maxTries) + ".")

		// sleep and double the delay between checks (exponential backoff)
		time.Sleep(delay)
		delay = delay + delay

		// try setting the check state again
		err = setCheckStatus(c, checkName, checkNamespace, healthCheckStatus)

		// count how many times we've retried
		tries++
	}

	return err
}

// ensureCheckResourceExists ensures that the healthcheck resource exists so that status can be updated.
func ensureCheckResourceExists(c client.Client, checkName string, checkNamespace string) error {
	if c == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	ctx := context.Background()
	nn := types.NamespacedName{Name: checkName, Namespace: checkNamespace}
	_, err := khapi.GetCheck(ctx, c, nn)
	if err != nil {
		if apierrors.IsNotFound(err) {
			hc := &khapi.HealthCheck{ObjectMeta: metav1.ObjectMeta{Name: checkName, Namespace: checkNamespace}}
			err = khapi.CreateCheck(ctx, c, hc)
			if err != nil {
				return fmt.Errorf("failed to create healthcheck %s/%s: %w", checkNamespace, checkName, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get healthcheck %s/%s: %w", checkNamespace, checkName, err)
	}
	return nil
}

// setCheckStatus sets the status on the healthcheck custom resource.
func setCheckStatus(c client.Client, checkName string, checkNamespace string, incoming *khapi.HealthCheckStatus) error {
	if c == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	ctx := context.Background()
	nn := types.NamespacedName{Name: checkName, Namespace: checkNamespace}
	khCheck, err := khapi.GetCheck(ctx, c, nn)
	if err != nil {
		return fmt.Errorf("failed to get healthcheck: %w", err)
	}

	// Merge the incoming status onto the existing status to avoid wiping
	// derived fields like lastRunUnix and runDuration. Only update fields
	// that the reporting pod controls.
	if incoming != nil {
		// core health fields
		khCheck.Status.OK = incoming.OK
		khCheck.Status.Errors = incoming.Errors

		// namespace is optional in reports; fall back to object namespace
		if incoming.Namespace != "" {
			khCheck.Status.Namespace = incoming.Namespace
		}
		if khCheck.Status.Namespace == "" {
			khCheck.Status.Namespace = checkNamespace
		}

		// clearing currentUUID when a run completes is critical for re-scheduling
		khCheck.Status.CurrentUUID = incoming.CurrentUUID

		// track consecutive failures similarly to internal controller helpers
		if incoming.OK {
			khCheck.Status.ConsecutiveFailures = 0
		} else if len(incoming.Errors) > 0 {
			khCheck.Status.ConsecutiveFailures++
		}
	} else {
		if khCheck.Status.Namespace == "" {
			khCheck.Status.Namespace = checkNamespace
		}
	}

	err = khapi.UpdateCheck(ctx, c, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update healthcheck status: %w", err)
	}

	return nil
}
