package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	kuberhealthycheckv2 "github.com/kuberhealthy/crds/api/v2"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/envs"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/health"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/metrics"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

const statusPageHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8" />
<title>Kuberhealthy Status</title>
<style>
body{font-family:sans-serif;}
.check{margin-bottom:0.5em;}
</style>
<script>
async function refresh(){
  try{
    const resp = await fetch('/json');
    const data = await resp.json();
    const container = document.getElementById('checks');
    container.innerHTML = '';
    const details = data.CheckDetails || {};
    Object.keys(details).forEach(function(name){
      const st = details[name];
      var icon = st.ok ? '✅' : '❌';
      if (st.podName){ icon = '⏳'; }
      var lastRun = st.lastRunUnix ? new Date(st.lastRunUnix*1000).toLocaleString() : 'never';
      var statusText = 'OK';
      if (!st.ok && st.errors && st.errors.length > 0){
        statusText = st.errors.join('; ');
      }
      var div = document.createElement('div');
      div.className = 'check';
      div.textContent = icon + ' ' + name + ' - Last Run: ' + lastRun + ' - ' + statusText;
      container.appendChild(div);
    });
  }catch(e){ console.error('failed to fetch status', e); }
}
setInterval(refresh,2000); window.onload = refresh;
</script>
</head>
<body>
<h1>Kuberhealthy Checks</h1>
<div id="checks"></div>
</body>
</html>`

// newServeMux configures and returns a mux with all web handlers mounted.
func newServeMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if err := prometheusMetricsHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := healthCheckHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		if err := healthCheckHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		if err := checkReportHandler(w, r); err != nil {
			log.Errorln("checkStatus endpoint error:", err)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if err := checkReportHandler(w, r); err != nil {
				log.Errorln("checkStatus endpoint error:", err)
			}
			return
		}
		if err := statusPageHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	return mux
}

// StartWebServer starts a JSON status web server at the specified listener.
func StartWebServer() error {
	mux := newServeMux()
	log.Infoln("Starting web services on port", GlobalConfig.ListenAddress)
	return http.ListenAndServe(GlobalConfig.ListenAddress, mux)
}

// statusPageHandler serves a basic HTML page that polls the JSON endpoint
// to show the status of all configured checks.
func statusPageHandler(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write([]byte(statusPageHTML))
	if err != nil {
		log.Warningln("Error writing status page:", err)
	}
	return err
}

// PodReportInfo holds info about an incoming IP to the external check reporting endpoint
type PodReportInfo struct {
	Name      string
	UUID      string
	Namespace string
}

// function variables allow tests to stub dependencies
var (
	validateUsingRequestHeaderFunc  = validateUsingRequestHeader
	validatePodReportBySourceIPFunc = validatePodReportBySourceIP
	storeCheckStateFunc             = storeCheckState
)

// validateExternalRequest calls the Kubernetes API to fetch details about a pod using a selector string.
// It validates that the pod is allowed to report the status of a check. The pod is also expected
// to have the environment variable KH_CHECK_NAME
func validateExternalRequest(ctx context.Context, selector string) (PodReportInfo, error) {

	var podUUID string
	var podCheckName string
	var podCheckNamespace string

	reportInfo := PodReportInfo{}

	// fetch the pod from the api using a specified selector. We keep retrying for some time to avoid kubernetes control
	// plane api race conditions wherein fast reporting pods are not found in pod listings
	pod, err := fetchPodBySelectorForDuration(ctx, selector, time.Minute)
	if err != nil {
		return reportInfo, err
	}

	// set the pod namespace and name from the returned metadata
	podCheckName = pod.Annotations[envs.KHCheckNameAnnotationKey]
	if len(podCheckName) == 0 {
		return reportInfo, errors.New("error finding check name annotation on calling pod with selector: " + selector)
	}

	podCheckNamespace = pod.GetNamespace()
	log.Debugln("Found check named", podCheckName, "in namespace", podCheckNamespace)

	// pile up all the env vars for searching
	var envVars []v1.EnvVar
	// we found our pod, lets return all its env vars from all its containers
	for _, container := range pod.Spec.Containers {
		envVars = append(envVars, container.Env...)
	}

	log.Debugln("Env vars found on pod with selector", selector, envVars)

	// validate that the environment variables we expect are in place based on the check's name and UUID
	// compared to what is in the khcheck custom resource
	var foundUUID bool
	for _, e := range envVars {
		log.Debugln("Checking environment variable on calling pod:", e.Name, e.Value)
		if e.Name == envs.KHRunUUID {
			log.Debugln("Found value on calling pod", selector, "value:", envs.KHRunUUID, e.Value)
			podUUID = e.Value
			foundUUID = true
		}
	}

	// verify that we found the UUID
	if !foundUUID {
		return reportInfo, errors.New("error finding environment variable on remote pod: " + envs.KHRunUUID)
	}

	// we know that we have a UUID and check name now, so lets check their validity.  First, we sanity check.
	if len(podCheckName) < 1 {
		return reportInfo, errors.New("pod check name was invalid or unset")
	}
	if len(podCheckNamespace) < 1 {
		return reportInfo, errors.New("pod check namespace was invalid or unset")
	}
	if len(podUUID) < 1 {
		return reportInfo, errors.New("pod uuid was invalid or unset")
	}

	// create a report to send back to the function invoker
	reportInfo.Name = podCheckName
	reportInfo.Namespace = podCheckNamespace
	reportInfo.UUID = podUUID

	// next, we check the uuid against the check name to see if this uuid is the expected one.  if it isn't,
	// we return an error
	whitelisted, err := isUUIDWhitelistedForCheck(podCheckName, podCheckNamespace, podUUID)
	if err != nil {
		return reportInfo, fmt.Errorf("failed to fetch whitelisted UUID for check with error: %w", err)
	}
	if !whitelisted {
		return reportInfo, errors.New("pod was not properly whitelisted for reporting status of check " + podCheckName + " with uuid " + podUUID + " and namespace " + podCheckNamespace)
	}

	return reportInfo, nil
}

// prometheusMetricsHandler is a handler for all prometheus metrics requests
func prometheusMetricsHandler(w http.ResponseWriter, r *http.Request) error {
	log.Infoln("Client connected to prometheus metrics endpoint from", r.RemoteAddr, r.UserAgent())
	state := getCurrentState([]string{})

	m := metrics.GenerateMetrics(state, GlobalConfig.PromMetricsConfig)
	// write summarized health check results back to caller
	_, err := w.Write([]byte(m))
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}

// healthCheckHandler returns the current status of checks loaded into Kuberhealthy
// as JSON to the client. Respects namespace requests via URL query parameters (i.e. /?namespace=default)
func healthCheckHandler(w http.ResponseWriter, r *http.Request) error {
	log.Infoln("Client connected to status page from", r.RemoteAddr, r.UserAgent())

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

	// fetch the current status from our khstate resources
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

	log.Println("webserver:", requestID, "Client connected to check report handler from", r.UserAgent())

	// Validate request using the kh-run-uuid header. If the header doesn't exist, or there's an error with validation,
	// validate using the pod's remote IP.
	log.Println("webserver:", requestID, "validating external check status report from its reporting kuberhealthy run uuid:", r.Header.Get("kh-run-uuid"))
	podReport, reportValidated, err := validateUsingRequestHeaderFunc(ctx, r)
	if err != nil {
		log.Println("webserver:", requestID, "Failed to look up pod by its kh-run-uuid header:", r.Header.Get("kh-run-uuid"), err)
	}

	// If the check uuid header is missing, attempt to validate using calling pod's source IP
	if !reportValidated {
		log.Println("webserver:", requestID, "validating external check status report from the pod's remote IP:", r.RemoteAddr)
		podReport, err = validatePodReportBySourceIPFunc(ctx, r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Println("webserver:", requestID, "Failed to look up pod by its IP:", r.RemoteAddr, err)
			return nil
		}
	}
	log.Println("webserver:", requestID, "Calling pod is", podReport.Name, "in namespace", podReport.Namespace)

	// append pod info to request id for easy check tracing in logs
	requestID = requestID + " (" + podReport.Namespace + "/" + podReport.Name + ")"

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

	// create a details object from our incoming status report before storing it as a khstate custom resource
	details := &kuberhealthycheckv2.KuberhealthyCheckStatus{}
	details.Errors = state.Errors
	details.OK = state.OK
	// details.RunDuration = checkRunDuration
	details.Namespace = podReport.Namespace
	details.CurrentUUID = podReport.UUID

	// since the check is validated, we can proceed to update the status now
	log.Println("webserver:", requestID, "Setting check with name", podReport.Name, "in namespace", podReport.Namespace, "to 'OK' state:", details.OK, "uuid", details.CurrentUUID)
	err = storeCheckStateFunc(podReport.Name, podReport.Namespace, details)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println("webserver:", requestID, "failed to store check state for %s: %w", podReport.Name, err)
		return fmt.Errorf("failed to store check state for %s: %w", podReport.Name, err)
	}

	// write ok back to caller
	w.WriteHeader(http.StatusOK)
	log.Println("webserver:", requestID, "Request completed successfully.")
	return nil
}

// fetchPodBySelectorForDuration attempts to fetch a pod by a specified selector repeatedly for the supplied duration.
// If the pod is found, then we return it.  If the pod is not found after the duration, we return an error
func fetchPodBySelectorForDuration(ctx context.Context, selector string, d time.Duration) (v1.Pod, error) {
	endTime := time.Now().Add(d)

	for {
		if time.Now().After(endTime) {
			return v1.Pod{}, errors.New("Failed to fetch source pod with selector " + selector + " after trying for " + d.String())
		}

		p, err := fetchPodBySelector(ctx, selector)
		if err != nil {
			log.Warningln("was unable to find calling pod with selector " + selector + " while watching for duration. Error: " + err.Error())
			time.Sleep(time.Second)
			continue
		}

		return p, err
	}
}

// getCheckStatus fetches the status section of a kuberhealthy check resource and returns it
func getCheckStatus(checkName string, checkNamespace string) (*kuberhealthycheckv2.KuberhealthyCheckStatus, error) {

	// TODO
	return &kuberhealthycheckv2.KuberhealthyCheckStatus{}, nil
}

// isUUIDWhitelistedForCheck determines if the supplied uuid is whitelisted for the
// check with the supplied name.  Only one UUID can be whitelisted at a time.
// Operations are not atomic.  Whitelisting prevents expired or invalidated pods from
// reporting into the status endpoint when they shouldn't be.
func isUUIDWhitelistedForCheck(checkName string, checkNamespace string, uuid string) (bool, error) {

	// get the item in question
	checkStatus, err := getCheckStatus(checkName, checkNamespace)
	if err != nil {
		return false, err
	}

	log.Debugln("Validating current UUID", checkStatus.CurrentUUID, "vs incoming UUID:", uuid)
	if checkStatus.CurrentUUID == uuid {
		return true, nil
	}
	return false, nil
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
	if KHController == nil {
		state.OK = false
		state.AddError("kuberhealthy controller not initialized")
		return state
	}

	ctx := context.Background()
	var checkList kuberhealthycheckv2.KuberhealthyCheckList
	var opts []client.ListOption
	if len(namespaces) == 1 {
		opts = append(opts, client.InNamespace(namespaces[0]))
	}
	if err := KHController.Client.List(ctx, &checkList, opts...); err != nil {
		state.OK = false
		state.AddError(err.Error())
		return state
	}

	for _, c := range checkList.Items {
		if len(namespaces) > 1 && !containsString(c.Namespace, namespaces) {
			continue
		}
		status := c.Status
		if status.Namespace == "" {
			status.Namespace = c.Namespace
		}
		state.CheckDetails[c.Name] = status
		if !status.OK {
			state.OK = false
			state.AddError(status.Errors...)
		}
	}

	return state
}

// validateUsingRequestHeader gets the header `kh-run-uuid` value from the request and forms a selector with it to
// validate that the request is coming from a kuberhealthy check pod
func validateUsingRequestHeader(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {

	var podReport PodReportInfo
	var err error
	if len(r.Header.Get("kh-run-uuid")) == 0 {
		return podReport, false, nil
	}
	selector := "kuberhealthy-run-id=" + r.Header.Get("kh-run-uuid")
	podReport, err = validateExternalRequest(ctx, selector)
	if err != nil {
		return podReport, false, err
	}
	return podReport, true, nil
}

// validatePodReportBySourceIP parses the remoteAddr from the request and forms a selector with the remote IP to
// validate that the request is coming from a kuberhealthy check pod
func validatePodReportBySourceIP(ctx context.Context, r *http.Request) (PodReportInfo, error) {

	var podReport PodReportInfo
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return podReport, err
	}
	selector := "status.podIP==" + ip + ",status.phase==Running"
	podReport, err = validateExternalRequest(ctx, selector)
	if err != nil {
		return podReport, err
	}
	return podReport, nil
}

// fetchPodBySelector fetches the pod by it's `kuberhealthy-run-id` label selector or by its `status.podIP` field selector
func fetchPodBySelector(ctx context.Context, selector string) (v1.Pod, error) {
	// var pod v1.Pod

	// podClient := kubernetesClient.CoreV1().Pods(TargetNamespace)

	// // Use either label selector or field selector depending on the selector string passed through
	// // LabelSelector: "kuberhealthy-run-id=" + uuid,
	// // FieldSelector: "status.podIP==" + remoteIP + ",status.phase==Running",
	// var listOptions metav1.ListOptions
	// if strings.Contains(selector, "kuberhealthy-run-id") {
	// 	listOptions = metav1.ListOptions{
	// 		LabelSelector: selector,
	// 	}
	// }

	// if strings.Contains(selector, "status.podIP") {
	// 	listOptions = metav1.ListOptions{
	// 		FieldSelector: selector,
	// 	}
	// }

	// podList, err := podClient.List(ctx, listOptions)
	// if err != nil {
	// 	return pod, errors.New("failed to fetch pod with selector " + selector + " with error: " + err.Error())
	// }

	// // ensure that we only got back one pod, because two means something awful has happened and 0 means we
	// // didnt find one
	// if len(podList.Items) == 0 {
	// 	return pod, errors.New("failed to find a pod with selector " + selector)
	// }
	// if len(podList.Items) > 1 {
	// 	return pod, errors.New("failed to fetch pod with selector " + selector + " - found two or more with same label")
	// }

	// // check if the pod has containers
	// if len(podList.Items[0].Spec.Containers) == 0 {
	// 	return pod, errors.New("failed to fetch environment variables from pod with selector" + selector + " - pod had no containers")
	// }

	// return podList.Items[0], nil

	return v1.Pod{}, nil
}

// storeCheckState stores the check state in its cluster CRD
func storeCheckState(checkName string, checkNamespace string, khcheck *kuberhealthycheckv2.KuberhealthyCheckStatus) error {

	// ensure the CRD resource exits
	// err := ensureStateResourceExists(checkName, checkNamespace, details.GetKHWorkload())
	// if err != nil {
	// 	return err
	// }

	// put the status on the CRD from the check
	// err := setCheckStateResource(checkName, checkNamespace, details)

	//TODO: Make this retry of updating custom resources repeatable
	//
	// We commonly see a race here with the following type of error:
	// "Error storing CRD state for check: pod-restarts in namespace kuberhealthy Operation cannot be fulfilled on khstates.comcast.github.io \"pod-restarts\": the object
	// has been modified; please apply your changes to the latest version and try again"
	//
	// If we see this error, we fetch the updated object, re-apply our changes, and try again
	delay := time.Duration(time.Second * 1)
	maxTries := 7
	tries := 0
	var err error
	for strings.Contains(err.Error(), "the object has been modified") {

		// if too many retires have occurred, we fail up the stack further
		if tries > maxTries {
			return fmt.Errorf("failed to update khstate for check %s in namespace %s after %d with error %w", checkName, checkNamespace, maxTries, err)
		}
		log.Infoln("Failed to update khstate for check because object was modified by another process.  Retrying in " + delay.String() + ".  Try " + strconv.Itoa(tries) + " of " + strconv.Itoa(maxTries) + ".")

		// sleep and double the delay between checks (exponential backoff)
		time.Sleep(delay)
		delay = delay + delay

		// try setting the check state again
		// err = setCheckStateResource(checkName, checkNamespace, details)

		// count how many times we've retried
		tries++
	}

	return err
}
