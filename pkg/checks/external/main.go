// Package external is a kuberhealthy checker that acts as an operator
// to run external images as checks.
package external

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	apiv1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	khcheckv1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khcheck/v1"
	khjobv1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khjob/v1"
	khstatev1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khstate/v1"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/util"
)

// KHReportingURL is the environment variable used to tell external checks where to send their status updates
const KHReportingURL = "KH_REPORTING_URL"

// KHRunUUID is the environment variable used to tell external checks their check's UUID so that they
// can be de-duplicated on the server side.
const KHRunUUID = "KH_RUN_UUID"

// KHDeadline is the environment variable name for when checks must finish their runs by in unixtime
const KHDeadline = "KH_CHECK_RUN_DEADLINE"

// KHCheckNameAnnotationKey is the annotation which holds the check's name for later validation when the pod calls in
const KHCheckNameAnnotationKey = "comcast.github.io/check-name"

// KHPodNamespace is the namespace variable used to tell external checks their namespace to perform
// checks in.
const KHPodNamespace = "KH_POD_NAMESPACE"

// DefaultKuberhealthyReportingURL is the default location that external checks
// are expected to report into.
const DefaultKuberhealthyReportingURL = "http://kuberhealthy.kuberhealthy.svc.cluster.local/externalCheckStatus"

// kuberhealthyRunIDLabel is the pod label for the kuberhealthy run id value
const kuberhealthyRunIDLabel = "kuberhealthy-run-id"

// kuberhealthyCheckNameLabel is the label used to flag this pod as being managed by this checker
const kuberhealthyCheckNameLabel = "kuberhealthy-check-name"

// defaultTimeout is the default time a pod is allowed to run when this checker is created
const defaultTimeout = time.Minute * 15

// defaultShutdownGracePeriod is the default time a pod is given to shutdown gracefully
const defaultShutdownGracePeriod = time.Minute

// ErrPodRemovedExpectedly is a constant for the error when a pod is deleted expectedly during a check run
var ErrPodRemovedExpectedly = errors.New("pod deleted expectedly")

// ErrPodRemovedUnexpectedly is a constant for the error when a pod is deleted unexpectedly during a check run
var ErrPodRemovedUnexpectedly = errors.New("pod deleted unexpectedly")

// ErrPodDeletedBeforeRunning is a constant for the error when a pod is deleted before the check pod running
var ErrPodDeletedBeforeRunning = errors.New("the khcheck check pod is deleted, waiting for start failed")

// DefaultName is used when no check name is supplied
var DefaultName = "external-check"

// kubeConfigFile is the default location to check for a kubernetes configuration file
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// Namespace of Kuberhealthy pod. Used to help set ownerReference for created checker pods.
var kuberhealthyNamespace = "kuberhealthy"

// Checker implements a KuberhealthyCheck for external
// check execution and lifecycle management.
type Checker struct {
	CheckName                string // the name of this checker
	Namespace                string
	RunInterval              time.Duration // how often this check runs a loop
	RunTimeout               time.Duration // time check must run completely within
	KubeClient               *kubernetes.Clientset
	KHJobClient              *khjobv1.KHJobV1Client
	KHCheckClient            *khcheckv1.KHCheckV1Client
	KHStateClient            *khstatev1.KHStateV1Client
	PodSpec                  apiv1.PodSpec // the current pod spec we are using after enforcement of settings
	OriginalPodSpec          apiv1.PodSpec // the user-provided spec of the pod
	RunID                    string        // the uuid of the current run
	KuberhealthyReportingURL string        // the URL that the check should want to report results back to
	ExtraAnnotations         map[string]string
	ExtraLabels              map[string]string
	Node                     string             // the node the checker pod runs on
	currentCheckUUID         string             // the UUID of the current external checker running
	Debug                    bool               // indicates we should run in debug mode - run once and stop
	shutdownCTXFunc          context.CancelFunc // used to cancel things in-flight when shutting down gracefully
	shutdownCTX              context.Context    // a context used for shutting down the check gracefully
	wg                       sync.WaitGroup     // used to track background workers and processes
	hostname                 string             // hostname cache
	checkPodName             string             // the current unique checker pod name
	KHWorkload               khstatev1.KHWorkload
}

func init() {
	// Get namespace of Kuberhealthy pod. Used to help set ownerReference for created checker pods to proper
	// Kuberhealthy instance.
	kuberhealthyNamespace = util.GetInstanceNamespace(kuberhealthyNamespace)
	log.Infoln("Kuberhealthy is located in the", kuberhealthyNamespace, "namespace.")
}

// New creates a new external checker
func New(client *kubernetes.Clientset, checkConfig *khcheckv1.KuberhealthyCheck, khCheckClient *khcheckv1.KHCheckV1Client, khStateClient *khstatev1.KHStateV1Client, reportingURL string) *Checker {

	return NewCheck(client, checkConfig, khCheckClient, khStateClient, reportingURL)
}

func NewCheck(client *kubernetes.Clientset, checkConfig *khcheckv1.KuberhealthyCheck, khCheckClient *khcheckv1.KHCheckV1Client, khStateClient *khstatev1.KHStateV1Client, reportingURL string) *Checker {

	if len(checkConfig.Namespace) == 0 {
		checkConfig.Namespace = "kuberhealthy"
	}

	// build the checker object
	log.Debugf("Creating external check from check config: %+v \n", checkConfig)
	return &Checker{
		Namespace:                checkConfig.Namespace,
		KHCheckClient:            khCheckClient,
		KHStateClient:            khStateClient,
		CheckName:                checkConfig.Name,
		KuberhealthyReportingURL: reportingURL,
		RunTimeout:               defaultTimeout,
		ExtraAnnotations:         make(map[string]string),
		ExtraLabels:              make(map[string]string),
		OriginalPodSpec:          checkConfig.Spec.PodSpec,
		PodSpec:                  checkConfig.Spec.PodSpec,
		KubeClient:               client,
		KHWorkload:               khstatev1.KHCheck,
	}
}

func NewJob(client *kubernetes.Clientset, jobConfig *khjobv1.KuberhealthyJob, khJobClient *khjobv1.KHJobV1Client, khStateClient *khstatev1.KHStateV1Client, reportingURL string) *Checker {

	if len(jobConfig.Namespace) == 0 {
		jobConfig.Namespace = "kuberhealthy"
	}

	// build the checker object
	log.Debugf("Creating kuberhealthy job from job config: %+v \n", jobConfig)
	return &Checker{
		Namespace:                jobConfig.Namespace,
		KHJobClient:              khJobClient,
		KHStateClient:            khStateClient,
		CheckName:                jobConfig.Name,
		KuberhealthyReportingURL: reportingURL,
		ExtraAnnotations:         make(map[string]string),
		ExtraLabels:              make(map[string]string),
		OriginalPodSpec:          jobConfig.Spec.PodSpec,
		PodSpec:                  jobConfig.Spec.PodSpec,
		KubeClient:               client,
		KHWorkload:               khstatev1.KHJob,
	}
}

// regeneratePodName regenerates the name of this checker pod with a new name string
func (ext *Checker) regeneratePodName() {
	var err error

	// if no hostname in cache, fetch it from the OS
	if len(ext.hostname) == 0 {
		ext.hostname, err = os.Hostname()
		if err != nil {
			log.Fatalln("Could not determine my hostname with error:", err)
		}
	}

	// use the current unix timestamp as a string in the name formulation
	timeString := strconv.FormatInt(time.Now().Unix(), 10)

	// always lowercase the output
	ext.checkPodName = strings.ToLower(ext.CheckName + "-" + timeString)
}

// podName returns the name of the checker pod formulated from our hostname.  caches the hostname to reduce
// os hostname lookup calls. crashes the whole program if it cant find a hostname
func (ext *Checker) podName() string {
	return ext.checkPodName
}

// CurrentStatus returns the status of the check as of right now.  For the external checker, this means checking
// the khstatus resources on the cluster.
func (ext *Checker) CurrentStatus() (bool, []string) {

	// fetch the state from the resource
	state, err := ext.getKHState()
	if err != nil {
		if k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found") {
			// if the resource is not found, we default to "up" so not to throw alarms before the first run completes
			return true, []string{}
		}
		return false, []string{err.Error()} // any other errors in fetching state will be seen as the check being down
	}

	ext.log("length of error message slice:", len(state.Spec.Errors), state.Spec.Errors)
	if len(state.Spec.Errors) > 0 {
		ext.log("reporting check as OK=FALSE due to error messages > 0")
		return false, state.Spec.Errors
	}
	ext.log("reporting OK=TRUE due to error messages NOT > 0")
	return true, state.Spec.Errors
}

// Name returns the name of this check.  This name is used
// when creating a check status CRD as well as for the status
// output
func (ext *Checker) Name() string {
	return ext.CheckName
}

// CheckNamespace returns the namespace of this checker
func (ext *Checker) CheckNamespace() string {
	return ext.Namespace
}

// Interval returns the interval at which this check runs
func (ext *Checker) Interval() time.Duration {
	return ext.RunInterval
}

// Timeout returns the maximum run time for this check before it times out
func (ext *Checker) Timeout() time.Duration {
	return ext.RunTimeout
}

// Run executes the checker.  This is ran on each "tick" of
// the RunInterval and is executed by the Kuberhealthy checker
func (ext *Checker) Run(ctx context.Context, client *kubernetes.Clientset) error {

	// store the client in the checker
	ext.KubeClient = client

	// generate a new UUID for each run
	err := ext.setNewCheckUUID()
	if err != nil {
		return err
	}

	// run a check iteration
	ext.log("Running external check iteration")
	err = ext.RunOnce(ctx)

	// if the pod was removed, we skip this run gracefully
	if err != nil && err.Error() == ErrPodRemovedExpectedly.Error() {
		ext.log("pod was removed during check expectedly. skipping this run")
		return ErrPodRemovedExpectedly
	}

	// if the pod had an error, we set the error
	if err != nil {
		ext.log("Error with running external check:", err)
		return err
	}

	return nil
}

// getCheck gets the CRD information for this check from the kubernetes API.
func (ext *Checker) getCheck() (*khcheckv1.KuberhealthyCheck, error) {

	// get the item in question and return it along with any errors
	log.Debugln("Fetching check", ext.CheckName, "in namespace", ext.Namespace)
	checkConfig, err := ext.KHCheckClient.KuberhealthyChecks(ext.Namespace).Get(ext.CheckName, metav1.GetOptions{})
	if err != nil {
		return &khcheckv1.KuberhealthyCheck{}, err
	}

	return &checkConfig, err
}

// cleanup cleans up any running, pending, or unknown checker pods by evicting them. Succeeded or Failed pods are left alone for records
// if eviction fails, cleanup will attempt to forcefully kill the pod.
func (ext *Checker) cleanup(ctx context.Context) {
	ext.log("Evicting up any running pods with name", ext.podName())
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)

	// find all pods that are running still so we can evict them (not delete - for records)
	checkLabelSelector := kuberhealthyCheckNameLabel + " = " + ext.CheckName
	ext.log("eviction: looking for pods with the label", checkLabelSelector)
	podList, err := podClient.List(ctx, metav1.ListOptions{
		LabelSelector: checkLabelSelector,
	})

	// if we can't list pods, just give up gracefully
	if err != nil {
		ext.log("error when searching for checker pods to clean up", err)
		return
	}

	// evict all checker pods not status=Failed or status=Succeeded. Check for existence of pods afterwards and if eviction failed, forcefully attempt to kill the pods
	wg := sync.WaitGroup{}
	for _, p := range podList.Items {
		ext.log("finding pods that are not in status.phase=Failed or status.phase=Succeeded")
		ext.log("pod:", p.Name, "is in status:", p.Status.Phase)
		if p.Status.Phase == apiv1.PodPending || p.Status.Phase == apiv1.PodUnknown || p.Status.Phase == apiv1.PodRunning {
			wg.Add(1)
			go func(p apiv1.Pod) {
				defer wg.Done()
				ext.log("evicting pod", p.GetName(), "from namespace", p.GetNamespace())
				err := ext.evictPod(ctx, p.GetName(), p.GetNamespace())
				if err != nil {
					ext.log("error killing pod", p.GetName()+":", err)
				}
			}(p)
		}
	}
	wg.Wait()
}

// evictPod evicts a pod in a namespace. If eviction fails, it will check if the pod still exists and if so, attempt to kill and then return any errors.
// Uses a static 30s grace period.
func (ext *Checker) evictPod(ctx context.Context, podName string, podNamespace string) error {
	podClient := ext.KubeClient.CoreV1().Pods(podNamespace)
	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespace,
		},
	}
	err := podClient.Evict(ctx, eviction)
	if err != nil {
		ext.log("error when trying to cleanup/evict checker pod", podName, "in namespace", podNamespace+":", err)
		podExists, _ := util.PodNameExists(ext.KubeClient, podName, podNamespace)
		if podExists {
			err := util.PodKill(ext.KubeClient, podName, podNamespace, 30)
			if err != nil {
				ext.log("error killing pod", podName+":", err)
			}

		}
	}
	return err
}

// setUUID sets the current whitelisted UUID for the checker and updates it on the server.  If the
// check fails to run or be verified as set (by a susequent fetch), then it will try up to 9 times
// before returning an error.
func (ext *Checker) setUUID(uuid string) error {
	ext.log("Setting expected UUID to:", uuid)

	// fetch the existing khstate
	checkState, err := ext.getKHState()

	// if the fetch operation had an error, and it wasn't 'not found', we return an error
	if err != nil && !(k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found")) {
		return fmt.Errorf("error setting uuid for check %s %w", ext.CheckName, err)
	}

	// if the check was not found, we create a new khstate
	if err != nil && (k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found")) {
		ext.log("khstate did not exist for", ext.CheckName, ", so a default object will be created")
		details := khstatev1.NewWorkloadDetails(ext.KHWorkload)
		details.Namespace = ext.CheckNamespace()
		details.AuthoritativePod = ext.hostname
		details.OK = true
		details.CurrentUUID = uuid
		details.RunDuration = time.Duration(0).String()
		newState := khstatev1.NewKuberhealthyState(ext.CheckName, details)
		newState.Namespace = ext.Namespace
		ext.log("Creating khstate", newState.Name, newState.Namespace, "because it did not exist")
		_, err = ext.KHStateClient.KuberhealthyStates(ext.CheckNamespace()).Create(&newState)
		if err != nil {
			ext.log("failed to create a khstate after finding that it did not exist:", err)
			return err
		}

		// if we were able to make a new check, we are done
		return nil
	}

	// assign the new uuid to the fetched checkState
	checkState.Spec.CurrentUUID = uuid
	ext.log("Updating khstate to CurrentUUID:", checkState.Spec.CurrentUUID)
	_, err = ext.KHStateClient.KuberhealthyStates(ext.CheckNamespace()).Update(&checkState)
	if err != nil {
		log.Errorln("failed to update khstate CurrentUUID for check", checkState.Namespace, checkState.Name, "with error:", err)
	}

	// Verify that the new CurrentUUID is set with the server by fetching it.  If something goes
	// wrong, try to set it again up to maxTries times.
	tries := 0
	maxTries := 9
	for {
		if tries >= maxTries {
			return fmt.Errorf("failed to verify uuid %s was set for khstate %s after %d tries", checkState.Spec.CurrentUUID, checkState.Namespace+checkState.Name, tries)
		}
		tries++

		// fetch the check we just updated and ensure it set properly
		extCheck, err := ext.KHStateClient.KuberhealthyStates(ext.CheckNamespace()).Get(ext.Name(), metav1.GetOptions{})
		if err != nil {
			ext.log("error: failed to get khstate while verifying check uuid:", err)
			time.Sleep(time.Second)
			continue
		}
		if checkState.Spec.CurrentUUID == extCheck.Spec.CurrentUUID {
			ext.log(ext.Name() + " CurrentUUID update verified.")
			break
		}

		// in this circumstance, the khstate has been fetched, but the CurrentUUID value on it is not the one we set
		log.Warningln("during verification of the CurrentUUID being properly set on khstate", checkState.Namespace, checkState.Name, "UUID setting, we saw UUID", extCheck.Spec.CurrentUUID, "but expected UUID", checkState.Spec.CurrentUUID)
		time.Sleep(time.Second * 1)

		// Retry the fetch, CurrentUUID update, and set again
		ext.log("Retrying khstate update to set CurrentUUID:", checkState.Spec.CurrentUUID)
		checkState, err := ext.getKHState()
		if err != nil {
			log.Errorln("failed to fetch khstate for check", checkState.Namespace, checkState.Name, "with error:", err)
		}
		checkState.Spec.CurrentUUID = uuid
		_, err = ext.KHStateClient.KuberhealthyStates(ext.CheckNamespace()).Update(&checkState)
		if err != nil {
			log.Errorln("failed to update khstate CurrentUUID for check", checkState.Namespace, checkState.Name, "with error:", err)
		}
	}

	return err
}

// watchForCheckerPodShutdown watches for the pod running checks to be shut down.  This means that either the pod
// was evicted or manually killed by an admin.  In this case, we gracefully skip this run interval and log the event.
// Two channels are passed in.  shutdownEventNotifyC will send a notification when the checker pod is deleted and
// the context can be used to shutdown this checker gracefully.
func (ext *Checker) watchForCheckerPodDelete(ctx context.Context) chan error {

	ext.wg.Add(1)
	defer ext.wg.Done()

	// make a channel to abort the waiter with and start it in the background
	listOptions := metav1.ListOptions{
		LabelSelector: kuberhealthyRunIDLabel + "=" + ext.currentCheckUUID,
	}
	waitForDeleteChan := make(chan error)

	// start a new watcher with the api and give it a context for aborting early
	watcher, err := ext.startPodWatcher(ctx, listOptions)
	if err != nil {
		waitForDeleteChan <- fmt.Errorf("error creating pod watcher: %w", err)
	}

	// in the background, watch for the deleted event and notify the channel
	go func() {
		// watch for either an abort from upstream or removal of the selected pods
		select {
		case <-ctx.Done(): // graceful shutdown signal
			ext.log("pod shutdown monitor stopping gracefully")
		case <-ext.waitForDeletedEvent(watcher): // we saw the watched pod remove
			ext.log("pod shutdown monitor witnessed the checker pod being removed")
			waitForDeleteChan <- fmt.Errorf("pod shutdown monitor witnessed the checker pod being removed")
		}
		watcher.Stop()
	}()
	return waitForDeleteChan
}

// startPodWatcher tries to start a watcher with the specified list options.
func (ext *Checker) startPodWatcher(ctx context.Context, listOptions metav1.ListOptions) (watch.Interface, error) {

	// create the pod client used with the watcher
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)

	ext.log("creating a pod watcher")

	// start a new watch request
	return podClient.Watch(ctx, listOptions)
}

// waitForDeletedEvent watches a channel of results from a pod watch and notifies the returned channel when a
// removal is observed.  The supplied abort channel is for shutting down gracefully.
func (ext *Checker) waitForDeletedEvent(w watch.Interface) chan error {

	ext.wg.Add(1)
	defer ext.wg.Done()

	// restart the watcher repeatedly forever until we are told to shutdown
	ext.log("starting pod shutdown watcher")

	// watch events for a removal and read until the watcher empties entirely, but only notify once
	var err error
	outChan := make(chan error, 2)

	go func() {
		for e := range w.ResultChan() {
			ext.log("got a result when watching for pod to remove")
			switch e.Type {
			case watch.Modified: // this section is entirely informational
				ext.log("checker pod shutdown monitor saw a modified event.")
				p, ok := e.Object.(*apiv1.Pod)
				if !ok {
					ext.log("checker pod shutdown monitor saw a modified event and the object was not a pod. skipped.")
					break
				}
				ext.log("checker pod shutdown monitor saw a modified event. the pod changed to ", p.Status.Phase)
			case watch.Deleted: // we saw a deleted event, so notify upstream, but only once
				outChan <- nil
				return
			case watch.Error:
				ext.log("khcheck monitor saw an error event")
				o, ok := e.Object.(*metav1.Status)
				if !ok {
					err = fmt.Errorf("pod removal monitor had an error when watching for pod changes: " + o.String())
				} else {
					// ext.log("pod removal monitor had an unknown error when watching for pod changes", e)
					err = errors.New("unidentified error when watching for pod to be deleted")
				}
				outChan <- err
				return
			default:
				ext.log("pod removal monitor saw an irrelevant event type and ignored it:", e.Type)
			}
		}
		ext.log("wait for deleted event watcher has closed")
	}()

	// if the watch ends for any reason, we notify the listeners that our watch has ended
	ext.log("pod removal monitor ended unexpectedly")
	return outChan
}

// newError returns an error from the provided string by pre-pending the pod's name and namespace to it
func (ext *Checker) newError(s string) error {
	return errors.New(ext.CheckNamespace() + "/" + ext.Name() + ": " + s)
}

// RunOnce runs one check loop.  This creates a checker pod and ensures it starts,
// then ensures it changes to Running properly
func (ext *Checker) RunOnce(ctx context.Context) error {

	// create a context for this run
	ext.shutdownCTX, ext.shutdownCTXFunc = context.WithCancel(ctx)
	defer ext.shutdownCTXFunc()
	defer ext.cleanup(ctx)

	// regenerate the checker pod name with a new timestamp
	ext.regeneratePodName()

	// fetch the currently known lastReportTime for this check.  We will use this to know when the pod has
	// fully reported back with a status before exiting
	lastReportTime, err := ext.getCheckLastUpdateTime()
	if err != nil {
		return err
	}

	// validate the pod spec
	ext.log("Validating pod spec of external check")
	err = ext.validatePodSpec()
	if err != nil {
		return err
	}

	// init a timeout for this whole check
	ext.log("Timeout set to", ext.RunTimeout.String())
	deadline := time.Now().Add(ext.RunTimeout)
	timeoutChan := time.After(ext.RunTimeout)

	// condition the spec with the required labels and environment variables
	ext.log("Configuring spec of external check")
	err = ext.configureUserPodSpec(deadline)
	if err != nil {
		return ext.newError("failed to configure pod spec for Kubernetes from user specified pod spec: " + err.Error())
	}

	// sanity check our settings
	ext.log("Running sanity check on check parameters")
	err = ext.sanityCheck()
	if err != nil {
		return err
	}

	// waiting until all checker pods are gone...
	ext.log("Waiting for all existing pods to clean up")
	select {
	case <-timeoutChan:
		ext.log("timed out waiting for all existing pods to clean up")
		errorMessage := "failed to see pod cleanup within timeout"
		return ext.newError(errorMessage)
	case err = <-ext.waitForAllPodsToClear(ctx):
		if err != nil {
			errorMessage := "error waiting for pod to clean up: " + err.Error()
			ext.log(err.Error())
			return ext.newError(errorMessage)
		}
	case <-ext.shutdownCTX.Done():
		ext.log("shutting down check. aborting wait for all pods to clean up")
		return nil
	}
	ext.log("No checker pods exist.")

	// Spawn a waiter to see if the pod is deleted.  If this happens, we consider this check aborted cleanly
	// and continue on to the next interval because deletes normally occur from admin intervention.  We create
	// a unique context here because we want to cancel this watch before the check times out, but before
	// the end of this checker pod run.
	podShutdownWatchCtx, podShutdownWatchCtxCancel := context.WithCancel(ctx)
	podDeletedChan := ext.watchForCheckerPodDelete(podShutdownWatchCtx)
	defer podShutdownWatchCtxCancel()

	// Spawn kubernetes pod to run our external check
	ext.log("creating pod for external check:", ext.CheckName)
	ext.log("checker pod annotations and labels:", ext.ExtraAnnotations, ext.ExtraLabels)
	createdPod, err := ext.createPod(ctx)
	if err != nil {
		ext.log("error creating pod")
		return ext.newError("failed to create pod for checker: " + err.Error())
	}
	ext.log("Check", ext.Name(), "created pod", createdPod.Name, "in namespace", createdPod.Namespace)

	// watch for pod to start with a timeout (include time for a new node to be created)
	select {
	case <-timeoutChan: // were out of time
		ext.log("timed out waiting for pod to startup")
		return ext.newError("failed to see pod running within timeout")
	case err := <-podDeletedChan: // pod removed unexpectedly
		if err != nil {
			ext.log("error from pod shutdown watcher when watching for checker pod to start:", err.Error())
			ext.log("pod removed unexpectedly while waiting for pod to start running")
			return ErrPodRemovedUnexpectedly
		}
		ext.log("pod removed expectedly. pod shutdown monitor shutting down")
		return ErrPodRemovedExpectedly
	case err = <-ext.waitForPodStart(ctx): // pod started
		if err != nil {
			ext.cleanup(ctx)
			errorMessage := "error when waiting for pod to start: " + err.Error()
			ext.log(errorMessage)
			return ext.newError(errorMessage)
		}
		// flag the pod as running until this run ends
		ext.log("External check pod is running:", ext.podName())
	case <-ext.shutdownCTX.Done(): // shutdown signal
		ext.log("shutting down check. aborting watch for pod to start")
		return nil
	}

	// validate that the pod was able to update its khstate
	ext.log("Waiting for pod status to be reported from pod", ext.podName(), "in namespace", ext.Namespace)
	select {
	case <-timeoutChan: // out of time
		ext.log("timed out waiting for pod status to be reported")
		errorMessage := "timed out waiting for checker pod to report in"
		ext.log(errorMessage)
		return ext.newError(errorMessage)
	case err := <-podDeletedChan: // pod was removed
		if err != nil {
			ext.log("error from pod shutdown watcher when watching for checker pod to report results:", err.Error())
			ext.log("pod removed unexpectedly while waiting for pod to report results")
			return ErrPodRemovedUnexpectedly
		}
		ext.log("pod removed expectedly. pod shutdown monitor shutting down")
		return ErrPodRemovedExpectedly
	case err = <-ext.waitForPodStatusUpdate(lastReportTime): // pod reported in
		if err != nil {
			errorMessage := "found an error when waiting for pod status to update: " + err.Error()
			ext.log(errorMessage)
			return ext.newError(errorMessage)
		}
		ext.log("External check pod has reported status for this check iteration:", ext.podName())
	case <-ext.shutdownCTX.Done(): // shutdown signal
		ext.log("shutting down check. aborting wait for pod status to update")
		return nil
	}

	// after the pod reports in, we no longer want to watch for it to be removed, so we shut that waiter down
	podShutdownWatchCtxCancel()

	// validate that the pod stopped running properly (wait for the pod to exit)
	select {
	case <-timeoutChan: // out of time
		errorMessage := "timed out waiting for pod to exit"
		ext.log(errorMessage)
		return ext.newError(errorMessage)
	case err = <-ext.waitForPodExit(ctx): // pod stopped running
		ext.log("External check pod is done running:", ext.podName())
		if err != nil {
			errorMessage := "found an error when waiting for pod to exit: " + err.Error()
			ext.log(errorMessage, err)
			return ext.newError(errorMessage)
		}
	case <-ext.shutdownCTX.Done(): // shutdown signal
		ext.log("shutting down check. aborting wait for pod to be done running")
		return nil
	}

	ext.log("Run completed!")
	return nil
}

// log writes a normal InfoLn message output prefixed with this checker's name on it
func (ext *Checker) log(s ...interface{}) {
	log.Infoln(ext.currentCheckUUID+" "+ext.Namespace+"/"+ext.CheckName+":", s)
}

// sanityCheck runs a basic sanity check on the checker before running
func (ext *Checker) sanityCheck() error {
	if ext.Namespace == "" {
		return errors.New("check namespace can not be empty")
	}

	if ext.KubeClient == nil {
		return errors.New("kubeClient can not be nil")
	}

	if len(ext.PodSpec.Containers) == 0 {
		return errors.New("pod has no configured containers")
	}

	return nil
}

// getKHState gets the khstate for this check from the resource in the API server
func (ext *Checker) getKHState() (khstatev1.KuberhealthyState, error) {
	// fetch the khstate as it exists
	return ext.KHStateClient.KuberhealthyStates(ext.Namespace).Get(ext.CheckName, metav1.GetOptions{})
}

// getCheckLastUpdateTime fetches the last time the khstate custom resource for this check was updated
// as a time.Time.
func (ext *Checker) getCheckLastUpdateTime() (metav1.Time, error) {

	// fetch the state from the resource
	state, err := ext.getKHState()
	if err != nil && (k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found")) {
		return metav1.Time{}, nil
	}

	// return a zero time if the time is nil / zero
	if state.Spec.LastRun.IsZero() {
		return metav1.Time{}, nil
	}

	return *state.Spec.LastRun, err
}

// waitForPodStatusUpdate waits for a pod status to update from the specified time
func (ext *Checker) waitForPodStatusUpdate(lastUpdateTime metav1.Time) chan error {
	ext.log("waiting for pod to report in to status page...")

	// make the output channel we will return and close it whenever we are done
	outChan := make(chan error, 50)

	ext.wg.Add(1)
	go func() {

		defer ext.wg.Done()

		// watch events and return when the pod is in state running
		for {

			// wait between requests to the api
			time.Sleep(time.Second * 5)
			ext.log("waiting for external checker pod to report in...")

			// if the context is canceled, we stop
			select {
			case <-ext.shutdownCTX.Done():
				ext.log("aborting wait for external checker pod to report in due to context cancellation")
				outChan <- nil
				return
			default:
			}

			// check if the pod has reported in
			hasReported, err := ext.podHasReportedInAfterTime(lastUpdateTime)
			if err != nil {
				ext.log("Error checking if checker pod has reported in since last update time:", err)
				time.Sleep(time.Second)
				continue
			}

			// if the pod has reported, we indicate that upstream
			if hasReported {
				ext.log("saw pod update since", lastUpdateTime)
				outChan <- nil
				return
			}
			ext.log("have not yet seen pod update since", lastUpdateTime)
		}
	}()

	return outChan
}

// podHasReportedInAfterTime indicates if a pod has reported a state since the supplied timestamp
func (ext *Checker) podHasReportedInAfterTime(t metav1.Time) (bool, error) {
	// fetch the lastUpdateTime from the khstate as of right now
	currentUpdateTime, err := ext.getCheckLastUpdateTime()
	if err != nil {
		return false, err
	}

	// if the pod has updated, then we return and were done waiting
	ext.log("Last report time was:", t, "vs", currentUpdateTime)
	if currentUpdateTime.After(t.Time) {
		return true, nil
	}

	return false, nil
}

// waitForAllPodsToClear waits for all pods to clear up and be gone
func (ext *Checker) waitForAllPodsToClear(ctx context.Context) chan error {

	ext.log("waiting for pod to clear")

	// make the output channel we will return and close it whenever we are done
	outChan := make(chan error, 2)

	// setup a pod watching client for our current KH pod
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)

	ext.wg.Add(1)
	go func() {

		defer ext.wg.Done()

		// watch events and return when the pod is in state running
		for {
			log.Debugln("Waiting for checker pod", ext.podName(), "to clear...")

			// wait between requests
			time.Sleep(time.Second * 5)

			// if the context is canceled, we stop
			select {
			case <-ext.shutdownCTX.Done():
				outChan <- nil
				return
			default:
			}

			// fetch the pod by name
			p, err := podClient.Get(ctx, ext.podName(), metav1.GetOptions{})

			// if we got a "not found" message, then we are done.  This is the happy path.
			if err != nil {
				if k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found") {
					ext.log("all pods cleared")
					outChan <- nil
					return
				}

				// if the error was anything else, we return it upstream
				outChan <- err
				return
			}
			ext.log("pod", ext.podName(), "still exists with status", p.Status.Phase, p.Status.Message, "- waiting for removal...")
		}
	}()

	return outChan
}

// waitForPodExit returns a channel that notifies when the checker pod exits
func (ext *Checker) waitForPodExit(ctx context.Context) chan error {

	ext.log("waiting for pod to exit")

	// make the output channel we will return
	outChan := make(chan error, 50)

	// setup a pod watching client for our current KH pod
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)

	ext.wg.Add(1)
	go func() {

		defer ext.wg.Done()

		for {

			ext.log("starting pod exit watcher")
			// Eric Greer: This was moved away from a watch because the watch was not getting updates of pods shutting
			// down sometimes, causing false alerts that checker pods failed to stop.

			// start a new watch request
			pods, err := podClient.List(ctx, metav1.ListOptions{
				LabelSelector: kuberhealthyRunIDLabel + "=" + ext.currentCheckUUID,
			})

			// return the watch error as a channel if found
			if err != nil {
				outChan <- err
				return
			}

			// watch events and return when the pod is in state running
			var podExists bool
			for _, p := range pods.Items {

				// if the pod is running or pending, we consider it to "exist"
				if p.Status.Phase == apiv1.PodRunning || p.Status.Phase == apiv1.PodPending {
					podExists = true
					break
				}

			}

			// if the pod does not exist, our watch has ended.
			if !podExists {
				outChan <- nil
				return
			}

			// if the context is done, we break the checking loop and return cleanly
			select {
			case <-ext.shutdownCTX.Done():
				ext.log("external checker pod aborted due to check context being aborted")
				outChan <- nil
				return
			default:
				// context is not canceled yet, continue
			}

			time.Sleep(time.Second * 5) // sleep between polls
		}

	}()

	return outChan
}

// waitForPodStart returns a channel that notifies when the checker pod has advanced beyond 'Pending'
func (ext *Checker) waitForPodStart(ctx context.Context) chan error {

	ext.log("waiting for pod to be running")

	// make the output channel we will return
	outChan := make(chan error, 50)

	// setup a pod watching client for our current KH pod
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)

	ext.wg.Add(1)
	go func() {

		defer ext.wg.Done()

		// watch over and over again until we see our event or run out of time
		for {

			ext.log("starting pod running watcher")

			pods, err := podClient.List(ctx, metav1.ListOptions{
				LabelSelector: kuberhealthyRunIDLabel + "=" + ext.currentCheckUUID,
			})
			if err != nil {
				outChan <- err
				return
			}
			if pods.Size() == 0 {
				outChan <- ErrPodDeletedBeforeRunning
				return
			}
			// start watching
			watcher, err := podClient.Watch(ctx, metav1.ListOptions{
				LabelSelector: kuberhealthyRunIDLabel + "=" + ext.currentCheckUUID,
			})
			if err != nil {
				outChan <- err
				return
			}

			// watch events and return when the pod is in state running
			for e := range watcher.ResultChan() {

				ext.log("got an event while waiting for pod to start running")

				if e.Type == watch.Deleted {
					ext.log("the khcheck check pod is deleted, waiting for start failed!")
					outChan <- ErrPodDeletedBeforeRunning
					watcher.Stop()
					return
				}

				// try to cast the incoming object to a pod and skip the event if we cant
				p, ok := e.Object.(*apiv1.Pod)
				if !ok {
					ext.log("got a watch event for a non-pod object and ignored it")
					continue
				}

				// catch when the pod has an error image pull and return it as an error #201
				for _, containerStat := range p.Status.ContainerStatuses {
					if containerStat.State.Waiting == nil {
						continue
					}
					if containerStat.State.Waiting.Reason == "ErrImagePull" {
						ext.log("pod had an error image pull")
						outChan <- errors.New(containerStat.State.Waiting.Reason)
						watcher.Stop()
						return
					}
				}
				// read the status of this pod (its ours)
				ext.log("pod state is now:", string(p.Status.Phase))
				if p.Status.Phase == apiv1.PodRunning || p.Status.Phase == apiv1.PodFailed || p.Status.Phase == apiv1.PodSucceeded {
					ext.log("pod is now either running, failed, or succeeded")
					outChan <- nil
					watcher.Stop()
					return
				}

				// if the context is done, we break the loop and return
				select {
				case <-ext.shutdownCTX.Done():
					ext.log("external checker pod startup watch aborted due to check context being aborted")
					outChan <- nil
					watcher.Stop()
					return
				default:
					// context is not canceled yet, continue
				}
			}
		}
	}()

	return outChan
}

// validatePodSpec validates the user specified pod spec to ensure it looks like it
// has all the default configuration required
func (ext *Checker) validatePodSpec() error {

	// if containers are not set, then we return an error
	if len(ext.PodSpec.Containers) == 0 && len(ext.PodSpec.InitContainers) == 0 {
		return errors.New("no containers found in checks PodSpec")
	}

	// ensure that at least one container is defined
	if len(ext.PodSpec.Containers) == 0 {
		return errors.New("no containers found in checks PodSpec")
	}

	// ensure that all containers have an image set
	for _, c := range ext.PodSpec.Containers {
		if len(c.Image) == 0 {
			return errors.New("no image found in check's PodSpec for container " + c.Name + ".")
		}
	}

	return nil
}

// createPod prepares and creates the checker pod using the kubernetes API
func (ext *Checker) createPod(ctx context.Context) (*apiv1.Pod, error) {
	ext.log("Creating external checker pod named", ext.podName())
	p := &apiv1.Pod{}
	p.Annotations = make(map[string]string)
	p.Labels = make(map[string]string)
	p.Namespace = ext.Namespace
	p.Name = ext.podName()
	p.Spec = ext.PodSpec

	// enforce various labels and annotations on all checker pods created
	ext.addKuberhealthyLabels(p)

	// only set ownerReference for pods in the kuberhealthy namespace
	// as cross-namespace owner references are disabled by design
	if p.Namespace == kuberhealthyNamespace {

		// Get ownerReference for the kuberhealthy pod
		ownerRef, err := util.GetOwnerRef(ext.KubeClient, kuberhealthyNamespace)
		if err != nil {
			return nil, errors.New("Failed to getOwnerReference for pod: " + p.Name + ", err: " + err.Error())
		}

		// Set ownerReference on checker pods in kuberhealthy namespace
		p.OwnerReferences = ownerRef
	}

	return ext.KubeClient.CoreV1().Pods(ext.Namespace).Create(ctx, p, metav1.CreateOptions{})
}

// configureUserPodSpec configures a user-specified pod spec with
// the unique and required fields for compatibility with an external
// kuberhealthy check.  Required environment variables and settings
// overwrite user-specified values.
func (ext *Checker) configureUserPodSpec(deadline time.Time) error {

	// start with a fresh spec each time we regenerate the spec
	ext.PodSpec = ext.OriginalPodSpec

	// specify environment variables that need applied.  We apply environment
	// variables that set the report-in URL of kuberhealthy along with
	// the unique run ID of this pod
	overwriteEnvVars := []apiv1.EnvVar{
		{
			Name:  KHReportingURL,
			Value: ext.KuberhealthyReportingURL,
		},
		{
			Name:  KHRunUUID,
			Value: ext.currentCheckUUID,
		},
		{
			Name:  KHDeadline,
			Value: strconv.FormatInt(deadline.Unix(), 10),
		},
		{
			Name: KHPodNamespace,
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}

	// apply overwrite env vars on every container in the pod
	for i := range ext.PodSpec.Containers {
		ext.PodSpec.Containers[i].Env = resetInjectedContainerEnvVars(ext.PodSpec.Containers[i].Env, []string{KHReportingURL, KHRunUUID, KHPodNamespace, KHDeadline})
		ext.PodSpec.Containers[i].Env = append(ext.PodSpec.Containers[i].Env, overwriteEnvVars...)
	}

	// enforce restart policy of never
	ext.PodSpec.RestartPolicy = apiv1.RestartPolicyNever

	// enforce namespace as namespace of this checker
	ext.Namespace = ext.CheckNamespace()

	return nil
}

// addKuberhealthyLabels adds the appropriate labels to a kuberhealthy
// external checker pod.
func (ext *Checker) addKuberhealthyLabels(pod *apiv1.Pod) {
	// make the labels map if it does not exist on the pod yet
	if pod == nil {
		pod = &apiv1.Pod{}
	}
	if pod.ObjectMeta.Labels == nil {
		pod.ObjectMeta.Labels = make(map[string]string)
	}

	// apply all extra labels to pod as specified by khcheck spec
	for k, v := range ext.ExtraLabels {
		pod.ObjectMeta.Labels[k] = v
	}

	// stack the kuberhealthy run id on top of the existing labels
	pod.ObjectMeta.Labels[kuberhealthyRunIDLabel] = ext.currentCheckUUID
	pod.ObjectMeta.Labels[kuberhealthyCheckNameLabel] = ext.CheckName
	pod.ObjectMeta.Labels["app"] = "kuberhealthy-check" // enforce a the label with an app name

	// ensure annotations map isnt nil
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = make(map[string]string)
	}

	// ensure all extra annotations are applied as specified in the khcheck
	for k, v := range ext.ExtraAnnotations {
		pod.ObjectMeta.Annotations[k] = v
	}

	// overwrite the check name annotation for use with calling pod validation
	pod.ObjectMeta.Annotations[KHCheckNameAnnotationKey] = ext.CheckName

}

// createCheckUUID creates a UUID that represents a single run of the external check
func (ext *Checker) setNewCheckUUID() error {
	uniqueID := uuid.New()
	ext.currentCheckUUID = uniqueID.String()
	log.Debugln("Generated new UUID for external check:", ext.currentCheckUUID)

	// set whitelist in check configuration CRD so only this
	// currently running pod can report-in with a status update
	return ext.setUUID(ext.currentCheckUUID)

}

// waitForShutdown waits for the external pod to shut down and notfies the caller
// of the result by sending an error or nil on the returned channel
func (ext *Checker) waitForShutdown(ctx context.Context) chan error {
	// repeatedly fetch the pod until its gone or the context
	// is canceled
	doneChan := make(chan error, 1)

	go func() {
		for {
			time.Sleep(time.Second * 5)
			exists, err := util.PodNameExists(ext.KubeClient, ext.checkPodName, ext.Namespace)
			if err != nil {
				ext.log("shutdown completed with error: ", err)
				doneChan <- err
				return
			}
			if !exists {
				ext.log("shutdown completed")
				doneChan <- nil
				return
			}

			// see if the context has expired yet and give up if so
			select {
			case <-ctx.Done():
				doneChan <- errors.New("timed out when waiting for pod to shutdown")
				return
			default:
			}
		}
	}()

	return doneChan
}

// Shutdown signals the checker to begin a shutdown and cleanup
func (ext *Checker) Shutdown() error {

	// cancel the context for this checker run
	if ext.shutdownCTXFunc != nil {
		ext.log("aborting context for this check due to shutdown call")
		ext.shutdownCTXFunc()
	}

	// make a context to track pod removal and cleanup
	ctx, ctxCancel := context.WithTimeout(context.Background(), ext.Timeout())
	defer ctxCancel()

	log.Debugln("Waiting for pod", ext.podName(), "to shutdown")

	select {
	case err := <-ext.waitForShutdown(ctx):
		if err != nil {
			ext.log("Error waiting for pod removal during shutdown:", err)
			return err
		}
		ext.log("Check using pod " + ext.podName() + " successfully shutdown.")
	case <-time.After(defaultShutdownGracePeriod):
		ext.log("Reached timeout:", defaultShutdownGracePeriod, "trying to shutdown pod:", ext.podName(), "Killing pod forcefully.")
		err := util.PodKill(ext.KubeClient, ext.podName(), ext.Namespace, 0)
		if err != nil {
			ext.log("Error force killing pod: ", ext.podName(), " Error:", err)
			return err
		}
		ext.log("Check using pod " + ext.podName() + " killed forcefully.")
	}
	return nil
}

// resetInjectedContainerEnvVars resets injected environment variables
func resetInjectedContainerEnvVars(podVars []apiv1.EnvVar, injectedVars []string) []apiv1.EnvVar {
	sanitizedVars := make([]apiv1.EnvVar, 0)
	for _, podVar := range podVars {
		if containsEnvVarName(podVar.Name, injectedVars) {
			continue
		}
		sanitizedVars = append(sanitizedVars, podVar)
	}
	return sanitizedVars
}

// containsEnvVarName returns a boolean value based on whether or not
// an env var is contained within a list
func containsEnvVarName(envVar string, injectedVars []string) bool {
	for _, injectedVar := range injectedVars {
		if envVar == injectedVar {
			return true
		}
	}
	return false
}
