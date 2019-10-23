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
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/Comcast/kuberhealthy/pkg/checks/daemonSet"
	"github.com/Comcast/kuberhealthy/pkg/checks/dnsStatus"
	"github.com/Comcast/kuberhealthy/pkg/checks/external"
	"github.com/Comcast/kuberhealthy/pkg/checks/external/status"
	"github.com/Comcast/kuberhealthy/pkg/checks/podRestarts"
	"github.com/Comcast/kuberhealthy/pkg/checks/podStatus"
	"github.com/Comcast/kuberhealthy/pkg/health"
	"github.com/Comcast/kuberhealthy/pkg/khcheckcrd"
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

// NewKuberhealthy creates a new kuberhealthy checker instance
func NewKuberhealthy() *Kuberhealthy {
	kh := &Kuberhealthy{}
	return kh
}

// setCheckExecutionError sets an execution error for a check name in
// its crd status
func (k *Kuberhealthy) setCheckExecutionError(checkName string, checkNamespace string, exErr error) {
	details := health.NewCheckDetails()
	check, err := k.getCheck(checkName, checkNamespace)
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
	err = k.storeCheckState(checkName, checkNamespace, details)
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
	if k.cancelChecksFunc != nil {
		k.cancelChecksFunc()
	}

	// call a shutdown on all checks concurrently
	var stopWG sync.WaitGroup
	for _, c := range k.Checks {
		stopWG.Add(1)
		go func() {
			log.Debugln("Check", c.Name(), "stopping...")
			err := c.Shutdown()
			if err != nil {
				log.Errorln("Error stopping check", c.Name(), err)
			}
			stopWG.Done()
		}()
	}

	// wait for all checks to stop cleanly
	stopWG.Wait()
	log.Debugln("All checks stopped.")
}

// Start inits Kuberhealthy checks and master monitoring
func (k *Kuberhealthy) Start() {

	log.Infoln("Loading initial check configuration...")
	kuberhealthy.configureChecks()

	// Start the web server and restart it if it crashes
	go kuberhealthy.StartWebServer()

	// if influxdb is enabled, configure it
	if enableInflux {
		k.configureInfluxForwarding()
	}

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
		case <-becameMasterChan: // we have become the current master instance and should run checks
			// reset checks and re-add from configuration settings
			log.Infoln("Became master. Reconfiguring and starting checks.")
			kuberhealthy.configureChecks()
			k.StartChecks()
		case <-lostMasterChan: // we are no longer master
			log.Infoln("Lost master. Stopping checks.")
			k.StopChecks()
		case <-externalChecksUpdateChan: // external check change detected
			log.Infoln("Witnessed a khcheck resource change...")

			// if we are master, stop checks
			if isMaster {
				log.Infoln("Reloading external check configurations due to resource update.")
				k.StopChecks()
			}

			log.Infoln("Reloading check configuration...")
			kuberhealthy.configureChecks()

			// start checks again if we are master
			if isMaster {
				k.StartChecks()
			}
		}
	}
}

// watchForKHCheckChanges watches for changes to khcheck objects and returns them through the specified channel
func (k *Kuberhealthy) watchForKHCheckChanges(c chan struct{}) {

	log.Debugln("Spawned watcher for KH check changes")

	for {
		log.Debugln("Starting a watch for khcheck object changes")

		// wait a second so we don't retry too quickly on error
		time.Sleep(time.Second)

		// start a watch on khcheck resources
		watcher, err := khCheckClient.Watch(metav1.ListOptions{})
		if err != nil {
			log.Errorln("error watching khcheck objects:", err)
			continue
		}

		// loop over results and return them to the calling channel until we hit an error, then close and restart
		for khc := range watcher.ResultChan() {
			switch khc.Type {
			case watch.Added:
				log.Debugln("khcheck monitor saw an added event")
				c <- struct{}{}
			case watch.Modified:
				log.Debugln("khcheck monitor saw a modified event")
				c <- struct{}{}
			case watch.Deleted:
				log.Debugln("khcheck monitor saw a deleted event")
				c <- struct{}{}
			case watch.Bookmark:
				log.Debugln("khcheck monitor saw a bookmark event and ignored it")
			case watch.Error:
				log.Debugln("khcheck monitor saw an error event")
				e := khc.Object.(*metav1.Status)
				log.Errorln("Error when watching for khcheck changes:", e.Reason)
				break
			default:
				log.Warningln("khcheck monitor saw an unknown event type and ignored it:", khc.Type)
			}
		}
		watcher.Stop()
	}
}

// monitorExternalChecks watches for changes to the external check CRDs
func (k *Kuberhealthy) monitorExternalChecks(notify chan struct{}) {

	// make a map of resource versions so we know when things change
	knownSettings := make(map[string]khcheckcrd.CheckConfig)

	// start watching for events to changes in the background
	c := make(chan struct{})
	go k.watchForKHCheckChanges(c)

	// each time  we see a change in our khcheck structs, we should look at every object to see if something has changed
	for {

		// wait for the change channel to detect a change before scanning again
		<-c
		log.Debugln("Change notification received. Scanning for external check changes...")

		// fetch all khcheck resources from all namespaces
		l, err := khCheckClient.List(metav1.ListOptions{}, checkCRDResource, "")
		if err != nil {
			log.Errorln("Error listing check configuration resources", err)
			continue
		}

		// this bool indicates if we should send a change signal to the channel
		var foundChange bool

		// if a khcheck has been deleted, then we signal for change and purge it from the knownSettings map.
		for mapName := range knownSettings {
			var existsInItems bool // indicates the item exists in the item listing
			for _, i := range l.Items {
				itemMapName := i.Namespace + "/" + i.Name
				if itemMapName == mapName {
					existsInItems = true
					break
				}
			}
			if !existsInItems {
				log.Debugln("Detected khcheck deletion for", mapName)
				delete(knownSettings, mapName)
				foundChange = true
			}
		}

		// check for changes or additions in the incoming data
		for _, i := range l.Items {
			mapName := i.Namespace + "/" + i.Name

			log.Debugln("Scanning khcheck CRD", mapName, "for changes since last seen...")

			if len(i.Namespace) < 1 {
				log.Warning("Got khcheck update from object with no namespace...")
				continue
			}
			if len(i.Name) < 1 {
				log.Warning("Got khcheck update from object with no name...")
				continue
			}

			// if we don't know about this check yet, just store the state and continue.  The check is already
			// loaded on the first check configuration run.
			_, exists := knownSettings[mapName]
			if !exists {
				log.Debugln("First time seeing khcheck of name", mapName)
				knownSettings[mapName] = i.Spec
				foundChange = true
			}

			// check if run interval has changed
			if knownSettings[mapName].RunInterval != i.Spec.RunInterval {
				log.Debugln("The khcheck run interval for", mapName, "has changed.")
				foundChange = true
			}

			// check if CheckConfig has changed (PodSpec)
			if !foundChange && !reflect.DeepEqual(knownSettings[mapName].PodSpec, i.Spec.PodSpec) {
				log.Debugln("The khcheck for", mapName, "has changed.")
				foundChange = true
			}

			// finally, update known settings before continuing to the next interval
			knownSettings[mapName] = i.Spec
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

	log.Debugln("Fetching khcheck configurations...")

	// list all checks from all namespaces
	l, err := khCheckClient.List(metav1.ListOptions{}, checkCRDResource, "")
	if err != nil {
		return err
	}

	log.Debugln("Found", len(l.Items), "external checks to load")

	// iterate on each check CRD resource and add it as a check
	for _, r := range l.Items {
		log.Debugln("Loading check CRD:", r.Name)

		log.Debugf("External check custom resource loaded: %v", r)

		// create a new kubernetes client for this external checker
		log.Infoln("Enabling external check:", r.Name)
		c := external.New(kubernetesClient, &r, khCheckClient, khStateClient, externalCheckReportingURL)

		// parse the run interval string from the custom resource and setup the run interval
		c.RunInterval, err = time.ParseDuration(r.Spec.RunInterval)
		if err != nil {
			log.Errorln("Error parsing duration for check", c.CheckName, "in namespace", c.Namespace, err)
			log.Errorln("Defaulting check to a runtime of ten minutes.")
			c.RunInterval = DefaultRunInterval
		}

		log.Debugln("RunInterval for check:", c.CheckName, "set to", c.RunInterval)

		// parse the user specified timeout if present
		c.RunTimeout = khcheckcrd.DefaultTimeout
		if len(r.Spec.Timeout) > 0 {
			c.RunTimeout, err = time.ParseDuration(r.Spec.Timeout)
			if err != nil {
				log.Errorln("Error parsing timeout for check", c.CheckName, "in namespace", c.Namespace, err)
				log.Errorln("Defaulting check to a timeout of", khcheckcrd.DefaultTimeout)
			}
		}

		log.Debugln("RunTimeout for check:", c.CheckName, "set to", c.RunTimeout)

		// add on extra annotations and labels
		c.ExtraAnnotations = r.Spec.ExtraAnnotations
		c.ExtraLabels = r.Spec.ExtraLabels

		// add the check into the checker
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
		watcher, err := kubernetesClient.CoreV1().Pods(podNamespace).Watch(metav1.ListOptions{
			LabelSelector: "app=kuberhealthy",
		})
		if err != nil {
			log.Errorln("error when attempting to watch for kuberhealthy pod changes:", err)
			continue
		}

		// on each update from the watch, we re-check our master status.
		for range watcher.ResultChan() {
			k.checkMasterStatus(becameMasterChan, lostMasterChan)
		}
	}
}

// checkMasterStatus checks the current status of this node as a master and if necessary, notifies
// the channels supplied.  Returns the new isMaster state bool.
func (k *Kuberhealthy) checkMasterStatus(becameMasterChan chan bool, lostMasterChan chan bool) {

	// determine if we are currently master or not
	masterNow, err := masterCalculation.IAmMaster(kubernetesClient)
	if err != nil {
		log.Errorln(err)
		return
	}

	// stop checks if we are no longer the master
	if masterNow && !isMaster {
		select {
		case becameMasterChan <- true:
		default:
		}
	}

	// start checks if we are now master
	if !masterNow && isMaster {
		select {
		case lostMasterChan <- true:
		default:
		}
	}

	// refresh global isMaster state
	isMaster = masterNow
}

// runCheck runs a check on an interval and sets its status each run
func (k *Kuberhealthy) runCheck(ctx context.Context, c KuberhealthyCheck) {

	// run on an interval specified by the package
	ticker := time.NewTicker(c.Interval())

	// run the check forever and write its results to the kuberhealthy
	// CRD resource for the check
	for {

		// break out if context cancels
		select {
		case <-ctx.Done():
			// we don't need to call a check shutdown here because the same func that cancels this context calls
			// shutdown on all the checks configured in the kuberhealthy struct.
			log.Infoln("Shutting down check run due to context cancellation:", c.Name(), "in namespace", c.CheckNamespace())
			return
		default:
		}

		// Run the check
		log.Infoln("Running check:", c.Name())
		err := c.Run(kubernetesClient)
		if err != nil {
			if strings.Contains(err.Error(), "pod deleted expectedly") {
				log.Infoln("Skipping this run due to expected pod removal before completion")
				<-ticker.C
			}
			// set any check run errors in the CRD
			k.setCheckExecutionError(c.Name(), c.CheckNamespace(), err)
			log.Errorln("Error running check:", c.Name(), "in namespace", c.CheckNamespace()+":", err)
			<-ticker.C
			continue
		}
		log.Debugln("Done running check:", c.Name(), "in namespace", c.CheckNamespace())

		// make a new state for this check and fill it from the check's current status
		details := health.NewCheckDetails()
		details.Namespace = c.CheckNamespace()
		details.OK, details.Errors = c.CurrentStatus()

		// send data to the metric forwarder if configured
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
				{c.Name() + "." + c.CheckNamespace(): checkStatus},
			}
			err := k.MetricForwarder.Push(metric, tags)
			if err != nil {
				log.Errorln("Error forwarding metrics", err)
			}
		}

		log.Infoln("Setting state of check", c.Name(), "in namespace", c.CheckNamespace(), "to", details.OK, details.Errors)

		// store the check state with the CRD
		err = k.storeCheckState(c.Name(), c.CheckNamespace(), details)
		if err != nil {
			log.Errorln("Error storing CRD state for check:", c.Name(), "in namespace", c.CheckNamespace(), err)
		}

		log.Infoln("Waiting for next run of check", c.Name(), "in namespace", c.CheckNamespace())
		<-ticker.C // wait for next run
	}
}

// storeCheckState stores the check state in its cluster CRD
func (k *Kuberhealthy) storeCheckState(checkName string, checkNamespace string, details health.CheckDetails) error {

	// ensure the CRD resource exits
	err := ensureStateResourceExists(checkName, checkNamespace)
	if err != nil {
		return err
	}

	// put the status on the CRD from the check
	err = setCheckStateResource(checkName, checkNamespace, details)
	if err != nil {
		return err
	}

	log.Debugln("Successfully updated CRD for check:", checkName, "in namespace", checkNamespace)
	return err
}

// StartWebServer starts a JSON status web server at the specified listener.
func (k *Kuberhealthy) StartWebServer() {
	log.Infoln("Configuring web server")
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		err := k.prometheusMetricsHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// Accept status reports coming from external checker pods
	http.HandleFunc("/externalCheckStatus", func(w http.ResponseWriter, r *http.Request) {
		err := k.externalCheckReportHandler(w, r)
		if err != nil {
			log.Errorln("externalCheckStatus endpoint error:", err)
		}
	})

	// Assign all requests to be handled by the healthCheckHandler function
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := k.healthCheckHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// start web server any time it exits
	for {
		log.Infoln("Starting web services on port", k.ListenAddr)
		err := http.ListenAndServe(k.ListenAddr, nil)
		if err != nil {
			log.Errorln("Web server error:", err)
		}
		time.Sleep(time.Second / 2)
	}
}

// PodReportIPInfo holds info about an incoming IP to the external check reporting endpoint
type PodReportIPInfo struct {
	Name      string
	UUID      string
	Namespace string
}

// validateExternalRequest calls the Kubernetes API to fetch details about a pod by it's source IP
// and then validates that the pod is allowed to report the status of a check.  The pod is expected
// to have the environment variables KH_CHECK_NAME and KH_RUN_UUID
func (k *Kuberhealthy) validateExternalRequest(remoteIPPort string) (PodReportIPInfo, error) {

	var podUUID string
	var podCheckName string
	var podCheckNamespace string

	reportInfo := PodReportIPInfo{}

	// break the port off the remoteIPPort incoming string
	ipPortString := strings.Split(remoteIPPort, ":")
	if len(ipPortString) == 0 {
		return reportInfo, errors.New("remote ip:port was blank")
	}
	ip := ipPortString[0]

	// fetch the pod from the api using its ip.  We keep retrying for some time to avoid kubernetes control plane
	// api race conditions wherein fast reporting pods are not found in pod listings
	pod, err := k.fetchPodByIPForDuration(ip, time.Second*10)
	if err != nil {
		return reportInfo, err
	}

	// set the pod namespace and name from the returned metadata
	podCheckName = pod.GetName()
	podCheckNamespace = pod.GetNamespace()
	log.Debugln("Found pod name", podCheckName, "in namespace", podCheckNamespace)

	// pile up all the env vars for searching
	var envVars []v1.EnvVar
	// we found our pod, lets return all its env vars from all its containers
	for _, container := range pod.Spec.Containers {
		envVars = append(envVars, container.Env...)
	}

	log.Debugln("Env vars found on pod with IP", ip, envVars)

	// validate that the environment variables we expect are in place based on the check's name and UUID
	// compared to what is in the khcheck custom resource
	var foundUUID bool
	for _, e := range envVars {
		log.Debugln("Checking environment variable on calling pod:", e.Name, e.Value)
		if e.Name == external.KHRunUUID {
			log.Debugln("Found value on calling pod", ip, "value:", external.KHRunUUID, e.Value)
			podUUID = e.Value
			foundUUID = true
		}
	}

	// verify that we found the UUID
	if !foundUUID {
		return reportInfo, errors.New("error finding environment variable on remote pod: " + external.KHRunUUID)
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
	whitelisted, err := k.isUUIDWhitelistedForCheck(podCheckName, podCheckNamespace, podUUID)
	if !whitelisted {
		return reportInfo, errors.New("pod was not properly whitelisted for reporting status of check " + podCheckName + " with uuid " + podUUID + " and namespace " + podCheckNamespace)
	}

	return reportInfo, nil
}

// fetchPodByIPForDuration attempts to fetch a pod by its IP repeatedly for the supplied duration.  If the pod is found,
// then we return it.  If the pod is not found after the duraiton, we return an error
func (k *Kuberhealthy) fetchPodByIPForDuration(remoteIP string, d time.Duration) (v1.Pod, error) {
	endTime := time.Now().Add(d)

	for {
		if time.Now().After(endTime) {
			return v1.Pod{}, errors.New("Failed to fetch source pod with IP " + remoteIP + " after trying for " + d.String())
		}

		p, err := k.fetchPodByIP(remoteIP)
		if err != nil {
			log.Warningln("was unable to find calling pod with remote IP", remoteIP, "while watching for duration")
			time.Sleep(time.Millisecond * 330)
			continue
		}

		return p, err
	}
}

// fetchPodByIP fetches the pod by it's IP address.
func (k *Kuberhealthy) fetchPodByIP(remoteIP string) (v1.Pod, error) {
	var pod v1.Pod

	// find the pod by its IP address
	podClient := kubernetesClient.CoreV1().Pods("")
	listOptions := metav1.ListOptions{
		FieldSelector: "status.podIP==" + remoteIP + ",status.phase==Running",
	}
	podList, err := podClient.List(listOptions)
	if err != nil {
		return pod, errors.New("failed to fetch pod with remote ip " + remoteIP + " with error: " + err.Error())
	}

	// log the fetched pod DEBUG
	// b, err := json.MarshalIndent(podList,"","\t")
	// if err != nil {
	// 	log.Panic(string(b))
	// }
	// log.Debugln(string(b))

	// ensure that we only got back one pod, because two means something awful has happened and 0 means we
	// didnt find one
	if len(podList.Items) == 0 {
		return pod, errors.New("failed to find a pod with the remote ip " + remoteIP)
	}
	if len(podList.Items) > 1 {
		return pod, errors.New("failed to fetch pod with remote ip " + remoteIP + " - found two or more with same ip")
	}

	// check if the pod has containers
	if len(podList.Items[0].Spec.Containers) == 0 {
		return pod, errors.New("failed to fetch environment variables from pod with remote ip " + remoteIP + " - pod had no containers")
	}

	return podList.Items[0], nil
}

func (k *Kuberhealthy) externalCheckReportHandlerLog(s ...interface{}) {
	log.Infoln(s...)
}

// externalCheckReportHandler handles requests coming from external checkers reporting their status.
// This endpoint checks that the external check report is coming from the correct UUID before recording
// the reported status of the corresponding external check.  This endpoint expects a JSON payload of
// the `State` struct found in the github.com/Comcast/kuberhealthy/pkg/health package.  The request
// causes a check of the calling pod's spec via the API to ensure that the calling pod is expected
// to be reporting its status.
func (k *Kuberhealthy) externalCheckReportHandler(w http.ResponseWriter, r *http.Request) error {
	// make a request ID for tracking this request
	requestID := uuid.New().String()

	k.externalCheckReportHandlerLog(requestID, "Client connected to check report handler from", r.RemoteAddr, r.UserAgent())

	// validate the calling pod to ensure that it has a proper KH_CHECK_NAME and KH_RUN_UUID
	k.externalCheckReportHandlerLog(requestID, "validating external check status report from: ", r.RemoteAddr)
	ipReport, err := k.validateExternalRequest(r.RemoteAddr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		k.externalCheckReportHandlerLog(requestID, "Failed to look up pod by IP:", r.RemoteAddr, err)
		return nil
	}
	k.externalCheckReportHandlerLog(requestID, "Calling pod is", ipReport.Name, "in namespace", ipReport.Namespace)

	// append pod info to request id for easy check tracing in logs
	requestID = requestID + " (" + ipReport.Namespace + "/" + ipReport.Name + ")"

	// ensure the client is sending a valid payload in the request body
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		k.externalCheckReportHandlerLog(requestID, "Failed to read request body:", err.Error(), r.RemoteAddr)
		return nil
	}
	log.Debugln("Check report body:", string(b))

	// decode the bytes into a status struct as used by the client
	state := status.Report{}
	err = json.Unmarshal(b, &state)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		k.externalCheckReportHandlerLog(requestID, "Failed to unmarshal state json:", err, r.RemoteAddr)
		return nil
	}
	log.Debugf("Check report after unmarshal: +%v\n", state)

	// ensure that if ok is set to false, then an error is provided
	if !state.OK {
		if len(state.Errors) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			k.externalCheckReportHandlerLog(requestID, "Client attempted to report OK false without any error strings")
			return nil
		}
		for _, e := range state.Errors {
			if len(e) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				k.externalCheckReportHandlerLog(requestID, "Client attempted to report a blank error string")
				return nil
			}
		}
	}

	// create a details object from our incoming status report before storing it as a khstate custom resource
	details := health.NewCheckDetails()
	details.Errors = state.Errors
	details.OK = state.OK
	details.Namespace = ipReport.Namespace

	// since the check is validated, we can proceed to update the status now
	k.externalCheckReportHandlerLog(requestID, "Setting check with name", ipReport.Name, "in namespace", ipReport.Namespace, "to 'OK' state:", details.OK)
	err = k.storeCheckState(ipReport.Name, ipReport.Namespace, details)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		k.externalCheckReportHandlerLog(requestID, "failed to store check state for %s: %w", ipReport.Name, err)
		return fmt.Errorf("failed to store check state for %s: %w", ipReport.Name, err)
	}

	// write ok back to caller
	w.WriteHeader(http.StatusOK)
	k.externalCheckReportHandlerLog(requestID, "Request completed successfully.")
	return nil
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
	log.Infoln("Client connected to prometheus metrics endpoint from", r.RemoteAddr, r.UserAgent())
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

// healthCheckHandler returns the current status of checks loaded into Kuberhealthy
// as JSON to the client.
func (k *Kuberhealthy) healthCheckHandler(w http.ResponseWriter, r *http.Request) error {
	log.Infoln("Client connected to status page from", r.RemoteAddr, r.UserAgent())

	// If a request body was supplied, throw an error to ensure that checks don't report into the wrong url
	body, err := ioutil.ReadAll(r.Body)
	if len(body) > 0 {
		log.Warningln("Unexpected body from status page request. Verify check is reporting to the right status url", r.RemoteAddr, r.RequestURI)
		w.WriteHeader(http.StatusBadRequest)
		return err
	}

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

// getCurrentState fetches the current state of all checks from their CRD objects and returns the summary as a
// health.State. Failures to fetch CRD state return an error.
func (k *Kuberhealthy) getCurrentState() (health.State, error) {
	// create a new set of state for this page render
	state := health.NewState()

	// fetch a client for the master calculation
	log.Debugln("Calculating current master pod")

	// calculate the current master and apply it to the status output
	currentMaster, err := masterCalculation.CalculateMaster(kubernetesClient)
	state.CurrentMaster = currentMaster
	if err != nil {
		return state, err
	}

	// loop over every check and apply the current state to the status return
	log.Debugln("Currently tracking", len(k.Checks), "check(s)")
	for _, c := range k.Checks {
		log.Debugln("Getting status of check for web request to status page:", c.Name())

		// get the state from the CRD that exists for this check
		checkDetails, err := getCheckState(c)
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

		// skip the check if it has never been run before.  This prevents checks that have not yet
		// run from showing in the status page.
		if len(checkDetails.AuthoritativePod) == 0 {
			log.Debugln("Output for", c.Name(), "hidden from status page due to blank authoritative pod")
			continue
		}

		// parse check status from CRD and add it to the status. Skip blank errors
		for _, e := range checkDetails.Errors {
			if len(strings.TrimSpace(e)) == 0 {
				log.Warningln("Skipped an error that was blank when adding check details to current state.")
				continue
			}
			state.AddError(e)
			if !checkDetails.OK {
				log.Debugln("Status page: Setting OK to false due to check details not being OK")
				state.OK = false
			}
		}

		state.CheckDetails[c.Name()] = checkDetails
	}
	return state, nil
}

// getCheck returns a Kuberhealthy check object from its name, returns an error otherwise
func (k *Kuberhealthy) getCheck(name string, namespace string) (KuberhealthyCheck, error) {
	for _, c := range k.Checks {
		if c.Name() == name && c.CheckNamespace() == namespace {
			return c, nil
		}
	}
	return nil, fmt.Errorf("could not find Kuberhealthy check with name %s", name)
}

// configureChecks removes all checks set in Kuberhealthy and reloads them
// based on the configuration options
func (k *Kuberhealthy) configureChecks() {
	log.Infoln("Loading check configuration...")

	// wipe all existing checks before we configure
	k.Checks = []KuberhealthyCheck{}

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
		log.Infoln("Enabling daemonset checker")
		kuberhealthy.AddCheck(ds)
	}

	// add pod restart checking if enabled
	if enablePodRestartChecks {
		log.Infoln("Enabling pod restart checker")
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
		log.Infoln("Enabling pod status checker")
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
	if enableDNSStatusChecks {
		log.Infoln("Enabling dns checker")
		kuberhealthy.AddCheck(dnsStatus.New(dnsEndpoints))
	}

	// check external check configurations
	if enableExternalChecks {
		log.Infoln("Enabling external checks...")
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
func (k *Kuberhealthy) isUUIDWhitelistedForCheck(checkName string, checkNamespace string, uuid string) (bool, error) {

	// get the item in question
	checkConfig, err := khCheckClient.Get(metav1.GetOptions{}, checkCRDResource, checkNamespace, checkName)
	if err != nil {
		return false, err
	}

	log.Debugln("Validating current UUID", checkConfig.Spec.CurrentUUID, "vs incoming UUID:", uuid)
	if checkConfig.Spec.CurrentUUID == uuid {
		return true, nil
	}
	return false, nil
}

// configureInfluxForwarding sets up initial influxdb metric sending
func (k *Kuberhealthy) configureInfluxForwarding() {

	// configure influxdb
	metricClient, err := configureInflux()
	if err != nil {
		log.Fatalln("Error setting up influx client:", err)
	}
	kuberhealthy.MetricForwarder = metricClient
}
