package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/envs"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/health"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/metrics"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

// StartWebServer starts a JSON status web server at the specified listener.
func StartWebServer() {
	log.Infoln("Configuring web server")

	// Serve metrics for our checks
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		err := prometheusMetricsHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// visit /json to see a json representation of all current checks for easy automation
	http.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		// otherwise show the status page
		err := healthCheckHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// Accept status reports coming from external checker pods. This is the old Endpoint
	// for reporting check status from Kuberhealthy V2.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := externalCheckReportHandler(w, r)
		if err != nil {
			log.Errorln("externalCheckStatus endpoint error:", err)
		}

	})

	// start web server and restart it any time it exits
	for {
		log.Infoln("Starting web services on port", GlobalConfig.ListenAddress)
		err := http.ListenAndServe(GlobalConfig.ListenAddress, nil)
		if err != nil {
			log.Errorln("Web server ERROR:", err)
		}
		time.Sleep(time.Second / 2)
	}
}

// PodReportInfo holds info about an incoming IP to the external check reporting endpoint
type PodReportInfo struct {
	Name      string
	UUID      string
	Namespace string
}

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

// externalCheckReportHandler handles requests coming from external checkers reporting their status.
// This endpoint checks that the external check report is coming from the correct UUID or pod IP before recording
// the reported status of the corresponding external check.  This endpoint expects a JSON payload of
// the `State` struct found in the github.com/kuberhealthy/kuberhealthy/v2/pkg/health package.  The request
// causes a check of the calling pod's spec via the API to ensure that the calling pod is expected
// to be reporting its status.
func externalCheckReportHandler(w http.ResponseWriter, r *http.Request) error {
	// make a request ID for tracking this request
	requestID := "web: " + uuid.New().String()

	ctx := r.Context()

	log.Println("webserver:", requestID, "Client connected to check report handler from", r.UserAgent())

	// Validate request using the kh-run-uuid header. If the header doesn't exist, or there's an error with validation,
	// validate using the pod's remote IP.
	log.Println("webserver:", requestID, "validating external check status report from its reporting kuberhealthy run uuid:", r.Header.Get("kh-run-uuid"))
	podReport, reportValidated, err := k.validateUsingRequestHeader(ctx, r)
	if err != nil {
		log.Println("webserver:", requestID, "Failed to look up pod by its kh-run-uuid header:", r.Header.Get("kh-run-uuid"), err)
	}

	// If the check uuid header is missing, attempt to validate using calling pod's source IP
	if !reportValidated {
		log.Println("webserver:", requestID, "validating external check status report from the pod's remote IP:", r.RemoteAddr)
		podReport, err = k.validatePodReportBySourceIP(ctx, r)
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

	checkRunDuration := time.Duration(0).String()
	khWorkload := determineKHWorkload(podReport.Name, podReport.Namespace)

	switch khWorkload {
	case khstatev2.KHCheck:
		checkDetails := k.stateReflector.CurrentStatus().CheckDetails
		checkRunDuration = checkDetails[podReport.Namespace+"/"+podReport.Name].RunDuration

		// create a details object from our incoming status report before storing it as a khstate custom resource
		details := khstatev2.NewWorkloadDetails(khWorkload)
		details.Errors = state.Errors
		details.OK = state.OK
		details.RunDuration = checkRunDuration
		details.Namespace = podReport.Namespace
		details.CurrentUUID = podReport.UUID

		// since the check is validated, we can proceed to update the status now
		log.Println("webserver:", requestID, "Setting check with name", podReport.Name, "in namespace", podReport.Namespace, "to 'OK' state:", details.OK, "uuid", details.CurrentUUID, details.GetKHWorkload())
		err = k.storeCheckState(podReport.Name, podReport.Namespace, details)
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
}

// fetchPodBySelectorForDuration attempts to fetch a pod by a specified selector repeatedly for the supplied duration.
// If the pod is found, then we return it.  If the pod is not found after the duration, we return an error
func fetchPodBySelectorForDuration(ctx context.Context, selector string, d time.Duration) (v1.Pod, error) {
	endTime := time.Now().Add(d)

	for {
		if time.Now().After(endTime) {
			return v1.Pod{}, errors.New("Failed to fetch source pod with selector " + selector + " after trying for " + d.String())
		}

		p, err := k.fetchPodBySelector(ctx, selector)
		if err != nil {
			log.Warningln("was unable to find calling pod with selector " + selector + " while watching for duration. Error: " + err.Error())
			time.Sleep(time.Second)
			continue
		}

		return p, err
	}
}

// isUUIDWhitelistedForCheck determines if the supplied uuid is whitelisted for the
// check with the supplied name.  Only one UUID can be whitelisted at a time.
// Operations are not atomic.  Whitelisting prevents expired or invalidated pods from
// reporting into the status endpoint when they shouldn't be.
func isUUIDWhitelistedForCheck(checkName string, checkNamespace string, uuid string) (bool, error) {

	// get the item in question
	checkState, err := khStateClient.KuberhealthyStates(checkNamespace).Get(checkName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	log.Debugln("Validating current UUID", checkState.Spec.CurrentUUID, "vs incoming UUID:", uuid)
	if checkState.Spec.CurrentUUID == uuid {
		return true, nil
	}
	return false, nil
}

// getCurrentState fetches the current state of all checks from requested namespaces
// their CRD objects and returns the summary as a health.State. Without a requested namespace,
// this will return the state of ALL found checks.
// Failures to fetch CRD state return an error.
func getCurrentState(namespaces []string) health.State {

	var currentState health.State
	if len(namespaces) != 0 {
		currentState = getCurrentStatusForNamespaces(namespaces)
	} else {
		currentState = k.stateReflector.CurrentStatus()
	}

	currentState.CurrentMaster = currentMaster
	if len(cfg.StateMetadata) != 0 {
		currentState.Metadata = cfg.StateMetadata
	}

	return currentState
}

// getCurrentState fetches the current state of all checks from the requested namespaces
// their CRD objects and returns the summary as a health.State.
// Failures to fetch CRD state return an error.
func getCurrentStatusForNamespaces(namespaces []string) health.State {
	// if there is are requested namespaces, then filter out checks from namespaces not matching those requested
	states := k.stateReflector.CurrentStatus()
	statesForNamespaces := states
	statesForNamespaces.Errors = []string{}
	statesForNamespaces.OK = true
	statesForNamespaces.CheckDetails = make(map[string]khstatev1.WorkloadDetails)
	statesForNamespaces.JobDetails = make(map[string]khstatev1.WorkloadDetails)
	if len(namespaces) != 0 {
		statesForNamespaces = validateCurrentStatusForNamespaces(states.CheckDetails, namespaces, statesForNamespaces, khstatev1.KHCheck)
		statesForNamespaces = validateCurrentStatusForNamespaces(states.JobDetails, namespaces, statesForNamespaces, khstatev1.KHJob)
	}

	log.Infoln("khState reflector returning current status on", len(statesForNamespaces.CheckDetails), "check khStates and", len(statesForNamespaces.JobDetails), "job khStates")
	return statesForNamespaces
}
