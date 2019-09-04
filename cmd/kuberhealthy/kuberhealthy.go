// Copyright 2018 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/Comcast/kuberhealthy/pkg/checks/componentStatus"
	"github.com/Comcast/kuberhealthy/pkg/checks/daemonSet"
	"github.com/Comcast/kuberhealthy/pkg/checks/dnsStatus"
	"github.com/Comcast/kuberhealthy/pkg/checks/external"
	"github.com/Comcast/kuberhealthy/pkg/checks/podRestarts"
	"github.com/Comcast/kuberhealthy/pkg/checks/podStatus"
	"github.com/Comcast/kuberhealthy/pkg/health"
	"github.com/Comcast/kuberhealthy/pkg/khcheckcrd"
	"github.com/Comcast/kuberhealthy/pkg/khstatecrd"
	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
	"github.com/Comcast/kuberhealthy/pkg/masterCalculation"
	"github.com/Comcast/kuberhealthy/pkg/metrics"
)

// Kuberhealthy represents the kuberhealthy server and its checks
type Kuberhealthy struct {
	Checks             []KuberhealthyCheck
	ListenAddr         string // the listen address, such as ":80"
	MetricForwarder    metrics.Client
	overrideKubeClient *kubernetes.Clientset
	cancelChecksFunc   context.CancelFunc // invalidates the context of all running checks
}

// KubeClient sets up a new kuberhealthy client if it does not exist yet
func (k *Kuberhealthy) KubeClient() (*kubernetes.Clientset, error) {

	// fetch a client if it does not exist
	if k.overrideKubeClient != nil {
		return k.overrideKubeClient, nil
	}

	// make a client if one does not exist
	return kubeClient.Create(kubeConfigFile)
}

// NewKuberhealthy creates a new kuberhealthy checker instance
func NewKuberhealthy() *Kuberhealthy {
	kh := &Kuberhealthy{}
	return kh
}

// setCheckExecutionError sets an execution error for a check name in
// its crd status
func (k *Kuberhealthy) setCheckExecutionError(checkName string, exErr error) {
	details := health.NewCheckDetails()
	check, err := k.getCheck(checkName)
	if err != nil {
		log.Errorln(err)
	}
	if check != nil {
		details.Namespace = check.CheckNamespace()
	}
	details.OK = false

	details.Errors = []string{"Check execution error: " + exErr.Error()}
	log.Debugln("Setting execution state of check", checkName, "to", details.OK, details.Errors)

	// store the check state with the CRD
	err = k.storeCheckState(checkName, details)
	if err != nil {
		log.Errorln("Was unable to write an execution error to the CRD status with error:", err)
	}
}

// AddCheck adds a check to Kuberhealthy.  Must be done before StartChecking
// is called.
func (k *Kuberhealthy) AddCheck(c KuberhealthyCheck) {
	k.Checks = append(k.Checks, c)
}

// Shutdown causes the kuberhealthy check group to shutdown gracefully
func (k *Kuberhealthy) Shutdown() {
	k.StopChecks()
	log.Debugln("All checks shutdown!")
	doneChan <- true
}

// StopChecks causes the kuberhealthy check group to shutdown gracefully.
// All checks are sent a shutdown command at the same time.
func (k *Kuberhealthy) StopChecks() {
	log.Infoln("Checks stopping...")
	k.cancelChecksFunc()
}

// Start inits Kuberhealthy checks and master monitoring
func (k *Kuberhealthy) Start() {

	// reset checks and re-add from configuration settings
	kuberhealthy.configureChecks()

	// find all the external checks from the khcheckcrd resources on the cluster and keep them in sync
	externalChecksUpdateChan := make(chan struct{})
	go k.monitorExternalChecks(externalChecksUpdateChan)

	// we use two channels to indicate when we gain or lose master status
	becameMasterChan := make(chan bool)
	lostMasterChan := make(chan bool)
	go k.masterStatusMonitor(becameMasterChan, lostMasterChan)

	// loop and select channels to do appropriate thing when master changes
	for {
		select {
		case <-becameMasterChan:
			log.Infoln("Became master. Starting checks.")
			k.StartChecks()
		case <-lostMasterChan:
			log.Infoln("Lost master. Stopping checks.")
			k.StopChecks()
		case <-externalChecksUpdateChan:
			log.Infoln("Reloading external check configurations due to resource update.")
			k.StopChecks()
			k.configureChecks() // reset and reconfigure all checks
			k.StartChecks()
		}
	}
}

// monitorExternalChecks watches for changes to the external check CRDs
func (k *Kuberhealthy) monitorExternalChecks(notify chan struct{}) {

	// make a map of resource versions so we know when things change
	resourceVersions := make(map[string]string)

	// start polling for check changes all the time.
	// TODO - watch is not implemented on the CRD package yet, but
	// when it is we should use that instead of polling.
	for {

		// rate limiting for watch restarts
		time.Sleep(checkCRDScanInterval)
		log.Infoln("Starting to monitor for external check configuration resource changes...")

		// make a new crd check client
		checkClient, err := khcheckcrd.Client(checkCRDGroup, checkCRDVersion, kubeConfigFile)
		if err != nil {
			log.Errorln("Error creating client for configuration resource listing", err)
			continue
		}

		l, err := checkClient.List(metav1.ListOptions{}, checkCRDResource)
		if err != nil {
			log.Errorln("Error listing check configuration resources", err)
			continue
		}

		// iterate on each check CRD resource for changes
		var foundChange bool
		for _, i := range l.Items {
			log.Debugln("Scanning check CRD", i.Name, "for changes...")
			r, err := checkClient.Get(metav1.GetOptions{}, checkCRDResource, i.Name)
			if err != nil {
				log.Errorln("Error getting check configuration resource:", err)
				continue
			}
			// if this is the first time seeing this resource, we don't notify of a change
			if len(resourceVersions[r.Name]) == 0 {
				resourceVersions[r.Name] = r.GetResourceVersion()
				log.Debugln("Initialized change monitoring for check CRD", r.Name, "at resource version", r.GetResourceVersion())
				continue
			}

			// if the resource has changed, we notify the notify channel and set the new resource version
			if resourceVersions[r.Name] != r.GetResourceVersion() {
				log.Infoln("Detected a change in external check CRD", r.Name, "to resource version", r.GetResourceVersion())
				resourceVersions[r.Name] = r.GetResourceVersion()
				foundChange = true
			}
		}

		// if a change was detected, we signal the notify channel
		if foundChange {
			log.Debugln("Signaling that a change was found in external check configuration")
			notify <- struct{}{}
		}
	}
}

// setExternalChecks syncs up the state of the external-checks installed in this
// Kuberhealthy struct.
func (k *Kuberhealthy) addExternalChecks() error {

	// make a new crd check client
	checkClient, err := khcheckcrd.Client(checkCRDGroup, checkCRDVersion, kubeConfigFile)
	if err != nil {
		return err
	}

	// list all checks
	l, err := checkClient.List(metav1.ListOptions{}, checkCRDResource)
	if err != nil {
		return err
	}

	// iterate on each check CRD resource
	for _, i := range l.Items {
		log.Debugln("Scanning check CRD", i.Name, "for changes...")
		r, err := checkClient.Get(metav1.GetOptions{}, checkCRDResource, i.Name)
		if err != nil {
			return err
		}

		// create a new KuberhealthyCheck and add it
		log.Infoln("Enabling external checker:", r.Name)
		kc, err := k.KubeClient()
		if err != nil {
			log.Fatalln("Could not fetch Kubernetes client for external checker:", err)
		}
		c := external.New(kc, &r.Spec.PodSpec)
		k.AddCheck(c)
	}

	return nil
}

// StartChecks starts all checks concurrently and ensures they stay running
func (k *Kuberhealthy) StartChecks() {
	log.Infoln("Checks starting...")

	// create a context for checks to abort with
	ctx, cancelFunc := context.WithCancel(context.Background())
	k.cancelChecksFunc = cancelFunc

	// start each check with this check group's context
	for _, c := range k.Checks {
		// start the check in its own routine
		go k.runCheck(ctx, c)
	}
}

// masterStatusMonitor calculates the master pod on a ticker.  When a
// change in master is determined that is relevant to this pod, a signal
// is sent down the appropriate became or lost channels
func (k *Kuberhealthy) masterStatusMonitor(becameMasterChan chan bool, lostMasterChan chan bool) {

	// continue reconnecting to the api to resume the pod watch if it ends
	for {

		// don't retry too fast
		time.Sleep(time.Second)

		// setup a pod watching client for kuberhealthy pods
		c, err := k.KubeClient()
		if err != nil {
			log.Errorln("attempted to fetch kube client, but found error:", err)
			continue
		}
		watcher, err := c.CoreV1().Pods(podNamespace).Watch(metav1.ListOptions{
			LabelSelector: "app=kuberhealthy",
		})
		if err != nil {
			log.Errorln("error when attempting to watch for kuberhealthy pod changes:", err)
			continue
		}

		// on each update from the watch, we re-check our master status.
		for range watcher.ResultChan() {
			k.checkMasterStatus(c, becameMasterChan, lostMasterChan)
		}
	}
}

// checkMasterStatus checks the current status of this node as a master and if necessary, notifies
// the channels supplied.  Returns the new isMaster state bool.
func (k *Kuberhealthy) checkMasterStatus(c *kubernetes.Clientset, becameMasterChan chan bool, lostMasterChan chan bool) {

	// determine if we are currently master or not
	becameMaster, err := masterCalculation.IAmMaster(c)
	if err != nil {
		log.Errorln(err)
		return
	}

	// stop checks if we are no longer the master
	if becameMaster && !isMaster {
		select {
		case lostMasterChan <- true:
		default:
		}
	}

	// start checks if we are now master
	if !becameMaster && isMaster {
		select {
		case becameMasterChan <- true:
		default:
		}
	}
}

// runCheck runs a check on an interval and sets its status each run
func (k *Kuberhealthy) runCheck(ctx context.Context, c KuberhealthyCheck) {

	// run on an interval specified by the package
	ticker := time.NewTicker(c.Interval())

	// run the check forever and write its results to the kuberhealthy
	// CRD resource for the check
	for {

		// break out if check channel is supposed to stop
		select {
		case <-ctx.Done():
			log.Debugln("Check", c.Name(), "stop signal received. Stopping check.")
			err := c.Shutdown()
			if err != nil {
				log.Errorln("Error stopping check", c.Name(), err)
			}
			return
		default:
		}

		log.Infoln("Running check:", c.Name())
		client, err := k.KubeClient()
		if err != nil {
			log.Errorln("Error creating Kubernetes client for check"+c.Name()+":", err)
			<-ticker.C
			continue
		}

		// Run the check
		err = c.Run(client)
		if err != nil {
			// set any check run errors in the CRD
			k.setCheckExecutionError(c.Name(), err)
			log.Errorln("Error running check:", c.Name(), err)
			<-ticker.C
			continue
		}
		log.Debugln("Done running check:", c.Name())

		// make a new state for this check and fill it from the check's current status
		details := health.NewCheckDetails()
		details.Namespace = c.CheckNamespace()
		details.OK, details.Errors = c.CurrentStatus()

		if k.MetricForwarder != nil {
			checkStatus := 0
			if details.OK {
				checkStatus = 1
			}

			tags := map[string]string{
				"KuberhealthyPod": details.AuthoritativePod,
				"Namespace":       c.CheckNamespace(),
				"Name":            c.Name(),
				"Errors":          strings.Join(details.Errors, ","),
			}
			metric := metrics.Metric{
				{c.Name() + "_status": checkStatus},
			}
			err := k.MetricForwarder.Push(metric, tags)
			if err != nil {
				log.Errorln("Error forwarding metrics", err)
			}
		}

		log.Infoln("Setting state of check", c.Name(), "to", details.OK, details.Errors)

		// store the check state with the CRD
		err = k.storeCheckState(c.Name(), details)
		if err != nil {
			log.Errorln("Error storing CRD state for check:", c.Name(), err)
		}
		<-ticker.C // wait for next run
	}
}

// storeCheckState stores the check state in its cluster CRD
func (k *Kuberhealthy) storeCheckState(checkName string, details health.CheckDetails) error {

	// make a new crd client
	client, err := khstatecrd.Client(statusCRDGroup, statusCRDVersion, kubeConfigFile)
	if err != nil {
		return err
	}

	// ensure the CRD resource exits
	err = ensureStateResourceExists(checkName, client)
	if err != nil {
		return err
	}

	// put the status on the CRD from the check
	err = setCheckStateResource(checkName, client, details)
	if err != nil {
		return err
	}

	log.Debugln("Successfully updated CRD for check:", checkName)
	return err
}

// StartWebServer starts a JSON status web server at the specified listener.
func (k *Kuberhealthy) StartWebServer() {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		err := k.prometheusMetricsHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// Accept status reports coming from external checker pods
	http.HandleFunc("/externalCheckStatus", func(w http.ResponseWriter, r *http.Request) {
		err := k.externalCheckStatusHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// Assign all requests to be handled by the healthCheckHandler function
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := k.healthCheckHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	log.Infoln("Starting web services on port", k.ListenAddr)
	err := http.ListenAndServe(k.ListenAddr, nil)
	if err != nil {
		log.Errorln(err)
	}
	os.Exit(1)
}

// validateExternalRequest calls the Kubernetes API to fetch details about a pod by it's source IP
// and then validates that the pod is allowed to report the status of a check.  The pod is expected
// to have the environment variables KH_CHECK_NAME and KH_RUN_UUID
func (k *Kuberhealthy) validateExternalRequest(remoteIP string) error {
	envVars, err := k.fetchPodEnvironmentVariablesByIP(remoteIP)
	if err != nil {
		return err
	}

	// validate that the environment variables we expect are in place based on the check's name and UUID
	// compared to what is in the khcheck custom resource
	var foundUUID bool
	var podUUID string
	var foundName bool
	var podCheckName string

	for _, e := range envVars {
		if e.Name == external.KHRunUUID {
			foundName = true
			podCheckName = e.Value
		}
		if e.Name == external.KHCheckName {
			foundName = true
			podUUID = e.Value
		}
	}

	// verify that we found the UUID
	if !foundUUID {
		return errors.New("error finding environment variable on remote pod: " + external.KHRunUUID)
	}
	// verify we found the check name
	if !foundName {
		return errors.New("error finding environment variable on remote pod: " + external.KHCheckName)
	}

	// we know that we have a UUID and check name now, so lets check their validity.  First, we sanity check.
	if len(podUUID) < 1 {
		return errors.New("pod uuid was invalid")
	}
	if len(podCheckName) < 1 {
		return errors.New("pod check name was invalid")
	}

	// next, we check the uuid against the check name to see if this uuid is the expected one.  if it isn't,
	// we return an error
	whitelisted, err := k.isUUIDWhitelistedForCheck(podCheckName, podUUID)
	if !whitelisted {
		return errors.New("pod was not properly whitelisted for reporting status of check " + podCheckName + " with uuid " + podUUID)
	}

	return nil
}

// fetchPodEnvironmentVariablesByIP fetches the environment variables for a pod by it's IP address.
func (k *Kuberhealthy) fetchPodEnvironmentVariablesByIP(remoteIP string) ([]v1.EnvVar, error) {
	var envVars []v1.EnvVar

	c, err := k.KubeClient()
	if err != nil {
		return envVars, errors.New("failed to create new kube client when finding pod environment variables:" + err.Error())
	}

	// fetch the pod by its IP address
	podClient := c.CoreV1().Pods("")
	listOptions := metav1.ListOptions{
		FieldSelector: "status.podIP==" + remoteIP,
	}
	podList, err := podClient.List(listOptions)
	if err != nil {
		return envVars, errors.New("failed to fetch pod with remote ip " + remoteIP + " with error: " + err.Error())
	}

	// ensure that we only got back one pod, because two means something awful has happened and 0 means we
	// didnt find one
	if len(podList.Items) == 0 {
		return envVars, errors.New("failed to fetch pod with remote ip " + remoteIP)
	}
	if len(podList.Items) > 1 {
		return envVars, errors.New("failed to fetch pod with remote ip " + remoteIP + " - found two or more with same ip")
	}

	// check if the pod has containers
	if len(podList.Items[0].Spec.Containers) == 0 {
		return envVars, errors.New("failed to fetch environment variables from pod with remote ip " + remoteIP + " - pod had no containers")
	}

	// we found our pod, lets return all its env vars from all its containers
	for _, container := range podList.Items[0].Spec.Containers {
		envVars = append(envVars, container.Env...)
	}

	return envVars, nil
}

// externalCheckStatusHandler takes status reports from external checkers,
// validates them to ensure they have the proper UUID expected by the external
// checker and then parses the response into the current check status.
func (k *Kuberhealthy) externalCheckStatusHandler(w http.ResponseWriter, r *http.Request) error {
}

// externalCheckReportHandler handles requests coming from external checkers reporting their status.
// This endpoint checks that the external check report is coming from the correct UUID before recording
// the reported status of the corresponding external check.  This endpoint expects a JSON payload of
// the `State` struct found in the github.com/Comcast/kuberhealthy/pkg/health package.  The request
// causes a check of the calling pod's spec via the API to ensure that the calling pod is expected
// to be reporting its status.
func (k *Kuberhealthy) externalCheckReportHandler(w http.ResponseWriter, r *http.Request) error {
	log.Infoln("Client connected to check report handler from", r.RemoteAddr, r.UserAgent())

	// validate the calling pod to ensure that it has a proper KH_CHECK_NAME and KH_RUN_UUID
	err := k.validateExternalRequest(r.RemoteAddr)

	// ensure the client is sending a valid payload in the request body
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorln("Failed to read request body:", err, r.RemoteAddr)
		return nil
	}

	// decode the bytes into a kuberhealthy health struct
	state := health.State{}
	err = json.Unmarshal(b, &state)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorln("Failed to unmarshal state json:", err, r.RemoteAddr)
		return nil
	}

	// write summarized health check results back to caller
	err = state.WriteHTTPStatusResponse(w)
	if err != nil {
		log.Warningln("Error writing report handler results to caller:", err)
	}
	return err
}

// writeHealthCheckError writes an error to the client when things go wrong in a health check handling
func (k *Kuberhealthy) writeHealthCheckError(w http.ResponseWriter, r *http.Request, err error, state health.State) {
	// if creating a CRD client fails, then write the error back to the user
	// as well as to the error log.
	state.OK = false
	state.AddError(err.Error())
	log.Errorln(err.Error())
	// write summarized health check results back to caller
	err = state.WriteHTTPStatusResponse(w)
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
}

func (k *Kuberhealthy) prometheusMetricsHandler(w http.ResponseWriter, r *http.Request) error {
	log.Infoln("Client connected to status page from", r.RemoteAddr, r.UserAgent())
	state, err := k.getCurrentState()
	if err != nil {
		err = metrics.WriteMetricError(w, state)
		if err != nil {
			return errors.New(err.Error() + " and " + err.Error())
		}
		return err
	}
	m := metrics.GenerateMetrics(state)
	// write summarized health check results back to caller
	_, err = w.Write([]byte(m))
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}

// healthCheckHandler runs health checks against kubernetes and
// returns a status output to a web request client
func (k *Kuberhealthy) healthCheckHandler(w http.ResponseWriter, r *http.Request) error {
	log.Infoln("Client connected to status page from", r.RemoteAddr, r.UserAgent())
	state, err := k.getCurrentState()
	if err != nil {
		k.writeHealthCheckError(w, r, err, state)
		return err
	}
	// write summarized health check results back to caller
	err = state.WriteHTTPStatusResponse(w)
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}

// getCurrentState fetches the current state of all checks from their CRD objects and returns the summary as a health.State. Failures to fetch CRD state return an error.
func (k *Kuberhealthy) getCurrentState() (health.State, error) {
	// create a new set of state for this page render
	state := health.NewState()

	// create a CRD client to fetch CRD states with
	khClient, err := khstatecrd.Client(statusCRDGroup, statusCRDVersion, kubeConfigFile)
	if err != nil {
		return state, err
	}

	// fetch a client for the master calculation
	kc, err := k.KubeClient()
	if err != nil {
		return state, err
	}

	// calculate the current master and apply it to the status output
	currentMaster, err := masterCalculation.CalculateMaster(kc)
	state.CurrentMaster = currentMaster
	if err != nil {
		return state, err
	}

	// loop over every check and apply the current state to the status return
	for _, c := range k.Checks {
		log.Debugln("Getting status of check for client:", c.Name())

		// get the state from the CRD that exists for this check
		checkDetails, err := getCheckState(c, khClient)
		if err != nil {
			errMessage := "System error when fetching status for check " + c.Name() + ":" + err.Error()
			log.Errorln(errMessage)
			// if there was an error getting the CRD, then use that for the check status
			// and set the check state to failed
			state.AddError(errMessage)
			log.Debugln("Status page: Setting OK to false due to an error in fetching crd state data")
			state.OK = false
			continue
		}

		// parse check status from CRD and add it to the status
		state.AddError(checkDetails.Errors...)
		if !checkDetails.OK {
			log.Debugln("Status page: Setting OK to false due to check details not being OK")
			state.OK = false
		}
		state.CheckDetails[c.Name()] = checkDetails
	}
	return state, nil
}

// getCheck returns a Kuberhealthy check object from its name, returns an error otherwise
func (k *Kuberhealthy) getCheck(name string) (KuberhealthyCheck, error) {
	for _, c := range k.Checks {
		if c.Name() == name {
			return c, nil
		}
	}
	return nil, fmt.Errorf("could not find Kuberhealthy check with name %s", name)
}

// configureChecks removes all checks set in Kuberhealthy and reloads them
// based on the configuration options
func (k *Kuberhealthy) configureChecks() {

	// wipe all existing checks before we configure
	k.Checks = []KuberhealthyCheck{}

	// if influxdb is enabled, configure it
	if enableInflux {
		// configure influxdb
		metricClient, err := configureInflux()
		if err != nil {
			log.Fatalln("Error setting up influx client:", err)
		}
		kuberhealthy.MetricForwarder = metricClient
	}

	// add componentstatus checking if enabled
	if enableComponentStatusChecks {
		kuberhealthy.AddCheck(componentStatus.New())
	}

	// add daemonset checking if enabled
	if enableDaemonSetChecks {
		ds, err := daemonSet.New()
		// allow the user to override the image used by the DSC - see #114
		if len(DSPauseContainerImageOverride) > 0 {
			log.Info("Setting DS pause container override image to:", DSPauseContainerImageOverride)
			ds.PauseContainerImage = DSPauseContainerImageOverride
		}
		if err != nil {
			log.Fatalln("unable to create daemonset checker:", err)
		}
		kuberhealthy.AddCheck(ds)
	}

	// add pod restart checking if enabled
	if enablePodRestartChecks {
		// Split the podCheckNamespaces into a []string
		namespaces := strings.Split(podCheckNamespaces, ",")
		for _, namespace := range namespaces {
			n := strings.TrimSpace(namespace)
			if len(n) > 0 {
				kuberhealthy.AddCheck(podRestarts.New(n))
			}
		}
	}

	// add pod status checking if enabled
	if enablePodStatusChecks {
		// Split the podCheckNamespaces into a []string
		namespaces := strings.Split(podCheckNamespaces, ",")
		for _, namespace := range namespaces {
			n := strings.TrimSpace(namespace)
			if len(n) > 0 {
				kuberhealthy.AddCheck(podStatus.New(n))
			}
		}
	}

	// add dns resolution checking if enabled
	if enableDnsStatusChecks {
		kuberhealthy.AddCheck(dnsStatus.New(dnsEndpoints))
	}

	// check external check configurations
	if enableExternalChecks {
		err := kuberhealthy.addExternalChecks()
		if err != nil {
			log.Errorln("Error loading external checks:", err)
		}
	}
}

// isUUIDWhitelistedForCheck determines if the supplied uuid is whitelisted for the
// check with the supplied name.  Only one UUID can be whitelisted at a time.
// Operations are not atomic.  Whitelisting prevents expired or invalidated pods from
// reporting into the status endpoint when they shouldn't be.
func (k *Kuberhealthy) isUUIDWhitelistedForCheck(checkName string, uuid string) (bool, error) {
	// make a new crd check client
	checkClient, err := khcheckcrd.Client(checkCRDGroup, checkCRDVersion, kubeConfigFile)
	if err != nil {
		return false, err
	}

	// get the item in question
	checkConfig, err := checkClient.Get(metav1.GetOptions{}, checkCRDResource, checkName)
	if err != nil {
		return false, err
	}

	if checkConfig.Spec.CurrentUUID == uuid {
		return true, nil
	}
	return false, nil
}
