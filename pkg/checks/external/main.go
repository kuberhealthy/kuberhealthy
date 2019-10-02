// Package external is a kuberhealthy checker that acts as an operator
// to run external images as checks.
package external

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/Comcast/kuberhealthy/pkg/khcheckcrd"
	"github.com/Comcast/kuberhealthy/pkg/khstatecrd"
)

// KHReportingURL is the environment variable used to tell external checks where to send their status updates
const KHReportingURL = "KH_REPORTING_URL"

// KHRunUUID is the environment variable used to tell external checks their check's UUID so that they
// can be de-duplicated on the server side.
const KHRunUUID = "KH_RUN_UUID"

// DefaultKuberhealthyReportingURL is the default location that external checks
// are expected to report into.
const DefaultKuberhealthyReportingURL = "http://kuberhealthy.kuberhealthy.svc.cluster.local/externalCheckStatus"

// kuberhealthyRunIDLabel is the pod label for the kuberhealthy run id value
const kuberhealthyRunIDLabel = "kuberhealthy-run-id"

// kuberhealthyCheckNameLabel is the label used to flag this pod as being managed by this checker
const kuberhealthyCheckNameLabel = "kuberhealthy-check-name"

// defaultTimeout is the default time a pod is allowed to run when this checker is created
const defaultTimeout = time.Minute * 15

// DefaultName is used when no check name is supplied
var DefaultName = "external-check"

// kubeConfigFile is the default location to check for a kubernetes configuration file
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// constants for using the kuberhealthy check CRD
const checkCRDGroup = "comcast.github.io"
const checkCRDVersion = "v1"
const checkCRDResource = "khchecks"
const stateCRDResource = "khstates"

// Checker implements a KuberhealthyCheck for external
// check execution and lifecycle management.
type Checker struct {
	CheckName                string // the name of this checker
	Namespace                string
	ErrorMessages            []string
	RunInterval              time.Duration // how often this check runs a loop
	RunTimeout               time.Duration // time check must run completely within
	KubeClient               *kubernetes.Clientset
	KHCheckClient            *khcheckcrd.KuberhealthyCheckClient
	KHStateClient            *khstatecrd.KuberhealthyStateClient
	PodSpec                  apiv1.PodSpec // the current pod spec we are using after enforcement of settings
	OriginalPodSpec          apiv1.PodSpec // the user-provided spec of the pod
	PodDeployed              bool          // indicates the pod exists in the API
	PodDeployedMu            sync.Mutex
	PodName                  string // the name of the deployed pod
	RunID                    string // the uuid of the current run
	KuberhealthyReportingURL string // the URL that the check should want to report results back to
	ExtraAnnotations         map[string]string
	ExtraLabels              map[string]string
	currentCheckUUID         string             // the UUID of the current external checker running
	Debug                    bool               // indicates we should run in debug mode - run once and stop
	cancelFunc               context.CancelFunc // used to cancel things in-flight
	ctx                      context.Context    // a context used for tracking check runs
}

// New creates a new external checker
func New(client *kubernetes.Clientset, checkConfig *khcheckcrd.KuberhealthyCheck, khCheckClient *khcheckcrd.KuberhealthyCheckClient, khStateClient *khstatecrd.KuberhealthyStateClient, reportingURL string) *Checker {
	if len(checkConfig.Namespace) == 0 {
		checkConfig.Namespace = "kuberhealthy"
	}

	log.Debugf("Creating external check from check config: %+v \n", checkConfig)

	// build the checker object
	return &Checker{
		ErrorMessages:            []string{},
		Namespace:                checkConfig.Namespace,
		KHCheckClient:            khCheckClient,
		KHStateClient:            khStateClient,
		CheckName:                checkConfig.Name,
		KuberhealthyReportingURL: reportingURL,
		RunTimeout:               defaultTimeout,
		ExtraAnnotations:         make(map[string]string),
		ExtraLabels:              make(map[string]string),
		PodName:                  checkConfig.Name,
		OriginalPodSpec:          checkConfig.Spec.PodSpec,
		PodSpec:                  checkConfig.Spec.PodSpec,
		KubeClient:               client,
	}
}

// cancel cancels the context of this checker to shut things down gracefully
func (ext *Checker) cancel() {
	if ext.cancelFunc == nil {
		return
	}
	ext.cancelFunc()
}

// CurrentStatus returns the status of the check as of right now
func (ext *Checker) CurrentStatus() (bool, []string) {
	if len(ext.ErrorMessages) > 0 {
		return false, ext.ErrorMessages
	}
	return true, ext.ErrorMessages
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
func (ext *Checker) Run(client *kubernetes.Clientset) error {

	// store the client in the checker
	ext.KubeClient = client

	// run a check iteration
	ext.log("Running external check iteration")
	err := ext.RunOnce()
	if err != nil {
		ext.log("Error with running external check:", err.Error())
		ext.setError(err.Error())
	}

	return nil
}

// getCheck gets the CRD information for this check from the kubernetes API.
func (ext *Checker) getCheck() (*khcheckcrd.KuberhealthyCheck, error) {

	// get the item in question and return it along with any errors
	log.Debugln("Fetching check", ext.CheckName, "in namespace", ext.Namespace)
	checkConfig, err := ext.KHCheckClient.Get(metav1.GetOptions{}, checkCRDResource, ext.Namespace, ext.CheckName)
	if err != nil {
		return &khcheckcrd.KuberhealthyCheck{}, err
	}

	return checkConfig, err
}

// cleanup cleans up any running pods
func (ext *Checker) cleanup() {
	ext.log("Cleaning up any deployed pods with name", ext.PodName)
	err := ext.deletePod()
	if err != nil && !strings.Contains(err.Error(), "not found") {
		ext.log("Error when cleaning up deployed pods: ", err.Error())
	}
}

// setUUID sets the current whitelisted UUID for the checker and updates it on the server
func (ext *Checker) setUUID(uuid string) error {
	log.Debugln("Set expected UUID to:", uuid)
	checkConfig, err := ext.getCheck()
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("error setting uuid for check %s %w", ext.CheckName, err)
		}
	}

	// update the check config and write it back to the struct
	checkConfig.Spec.CurrentUUID = uuid
	checkConfig.ObjectMeta.Namespace = ext.Namespace

	// update the resource with the new values we want
	_, err = ext.KHCheckClient.Update(checkConfig, checkCRDResource, ext.Namespace, ext.Name())
	return err
}

// RunOnce runs one check loop.  This creates a checker pod and ensures it starts,
// then ensures it changes to Running properly
func (ext *Checker) RunOnce() error {

	// create a context for this run
	ext.ctx, ext.cancelFunc = context.WithCancel(context.Background())

	// fetch the currently known lastReportTime for this check.  We will use this to know when the pod has
	// fully reported back with a status before exiting
	lastReportTime, err := ext.getCheckLastUpdateTime(ext.ctx)
	if err != nil {
		return err
	}

	// generate a new UUID for this run
	err = ext.setNewCheckUUID()
	if err != nil {
		return err
	}
	log.Debugln("UUID for external check", ext.Name(), "run:", ext.currentCheckUUID)

	// validate the pod spec
	ext.log("Validating pod spec of external check")
	err = ext.validatePodSpec()
	if err != nil {
		return err
	}

	// condition the spec with the required labels and environment variables
	ext.log("Configuring spec of external check")
	err = ext.configureUserPodSpec()
	if err != nil {
		return errors.New("failed to configure pod spec for Kubernetes from user specified pod spec: " + err.Error())
	}

	// sanity check our settings
	ext.log("Running sanity check on check parameters")
	err = ext.sanityCheck()
	if err != nil {
		return err
	}
	// cleanup all pods from this checker that should not exist right now (all of them)
	ext.log("Deleting any rogue check pods")
	err = ext.deletePod()
	if err != nil {
		errorMessage := "failed to clean up pods before starting external checker pod: " + err.Error()
		ext.log(errorMessage)
		return errors.New(errorMessage)
	}

	// init a timeout for this whole check
	ext.log("Timeout set to", ext.RunTimeout.String())
	timeoutChan := time.After(ext.RunTimeout)

	// waiting for all checker pods are gone...
	ext.log("Waiting for all existing pods to clean up")
	select {
	case <-timeoutChan:
		ext.cancel() // cancel the watch context, we have timed out
		ext.cleanup()
		errorMessage := "failed to see pod cleanup within timeout"
		ext.log(errorMessage + " " + err.Error())
		return errors.New(errorMessage)
	case err = <-ext.waitForAllPodsToClear(ext.ctx):
		if err != nil {
			ext.cancel() // cancel the watch context, we have timed out
			errorMessage := "error waiting for pod to clean up: " + err.Error()
			ext.log(err.Error())
			return errors.New(errorMessage)
		}
	}
	ext.log("No checker pods exist.")

	// Spawn kubernetes pod to run our external check
	ext.log("creating pod for external check:", ext.CheckName)
	createdPod, err := ext.createPod()
	if err != nil {
		return errors.New("failed to create pod for checker: " + err.Error())
	}
	ext.log("Check", ext.Name(), "created pod", createdPod.Name, "in namespace", createdPod.Namespace)

	// watch for pod to start with a timeout (include time for a new node to be created)
	select {
	case <-timeoutChan:
		ext.cancel() // cancel the watch context, we have timed out
		ext.cleanup()
		return errors.New("failed to see pod running within timeout")
	case err = <-ext.waitForPodRunning():
		if err != nil {
			ext.cancel() // cancel the watch context, we have timed out
			ext.cleanup()
			errorMessage := "error when waiting for pod to start: " + err.Error()
			ext.log(errorMessage + " " + err.Error())
			return errors.New(errorMessage)
		}
		ext.log("External check pod is running:", ext.PodName)
	}

	// flag the pod as running until this run ends
	ext.setPodDeployed(true)
	defer ext.setPodDeployed(false)

	// validate that the pod was able to update its khstate
	ext.log("Waiting for pod status to be reported from pod", ext.PodName, "in namespace", ext.Namespace)
	select {
	case <-timeoutChan:
		ext.cancel() // cancel the watch context, we have timed out
		ext.cleanup()
		errorMessage := "timed out waiting for checker pod to report in"
		ext.log(errorMessage)
		return errors.New(errorMessage)
	case err = <-ext.waitForPodStatusUpdate(lastReportTime, ext.ctx):
		if err != nil {
			ext.cancel() // cancel the watch context, we have timed out
			ext.cleanup()
			errorMessage := "found an error when waiting for pod status to update: " + err.Error()
			ext.log(errorMessage + " " + err.Error())
			return errors.New(errorMessage)
		}
		ext.log("External check pod has reported status for this check iteration:", ext.PodName)
	}

	// validate that the pod stopped running properly
	ext.log("Waiting for pod to exit")
	select {
	case <-timeoutChan:
		ext.cancel() // cancel the watch context, we have timed out
		ext.cleanup()
		errorMessage := "timed out waiting for pod to exit"
		ext.log(errorMessage)
		return errors.New(errorMessage)
	case err = <-ext.waitForPodExit():
		if err != nil {
			ext.cancel() // cancel the watch context, we have timed out
			errorMessage := "found an error when waiting for pod to exit: " + err.Error()
			ext.log(errorMessage + " " + err.Error())
			return errors.New(errorMessage)
		}
		ext.log("External check pod is done running:", ext.PodName)
	}

	ext.log("Run completed!")

	return nil
}

// Log writes a normal InfoLn message output prefixed with this checker's name on it
func (ext *Checker) log(s ...string) {
	log.Infoln(ext.Namespace+"/"+ext.CheckName+":", s)
}

// stopPod stops any pods running because of this external checker
func (ext *Checker) deletePod() error {
	ext.log("Deleting checker pods with name", ext.CheckName)
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)
	gracePeriodSeconds := int64(1)
	deletionPolicy := metav1.DeletePropagationForeground
	err := podClient.Delete(ext.PodName, &metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
		PropagationPolicy:  &deletionPolicy,
	})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}
	return nil
}

// sanityCheck runs a basic sanity check on the checker before running
func (ext *Checker) sanityCheck() error {
	if ext.Namespace == "" {
		return errors.New("check namespace can not be empty")
	}

	if ext.PodName == "" {
		return errors.New("pod name can not be empty")
	}

	if ext.KubeClient == nil {
		return errors.New("KubeClient can not be nil")
	}

	return nil
}

// getCheckLastUpdateTime fetches the last time the khstate custom resource for this check was updated
// as a time.Time.
func (ext *Checker) getCheckLastUpdateTime(ctx context.Context) (time.Time, error) {

	// fetch the khstate as it exists
	khstate, err := ext.KHStateClient.Get(metav1.GetOptions{}, stateCRDResource, ext.CheckName, ext.Namespace)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}

	return khstate.Spec.LastRun, nil

}

// waitForPodStatusUpdate waits for a pod status to update from the specified time
func (ext *Checker) waitForPodStatusUpdate(lastUpdateTime time.Time, ctx context.Context) chan error {
	ext.log("waiting for pod to report in to status page...")

	// make the output channel we will return and close it whenever we are done
	outChan := make(chan error, 2)
	defer close(outChan)

	// watch events and return when the pod is in state running
	for {

		// wait between requests to the api
		time.Sleep(time.Second * 5)
		ext.log("waiting for external checker pod to report in...")

		// if the context is canceled, we stop
		select {
		case <-ctx.Done():
			outChan <- errors.New("waiting for pod to clear was aborted by context cancellation")
			return outChan
		default:
		}

		// fetch the lastUpdateTime from the khstate as of right now
		currentUpdateTime, err := ext.getCheckLastUpdateTime(ctx)
		if err != nil {
			outChan <- err
			return outChan
		}

		// if the pod has updated, then we return and were done waiting
		log.Debugln("Last report time was:", lastUpdateTime, "vs", currentUpdateTime)
		if currentUpdateTime.After(lastUpdateTime) {
			return outChan
		}
	}
}

// waitForAllPodsToClear waits for all pods to clear up and be gone
func (ext *Checker) waitForAllPodsToClear(ctx context.Context) chan error {

	// make the output channel we will return and close it whenever we are done
	outChan := make(chan error, 2)
	defer close(outChan)

	// setup a pod watching client for our current KH pod
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)

	// watch events and return when the pod is in state running
	for {
		log.Debugln("Waiting for checker pod", ext.PodName, "to clear...")

		// wait between requests
		time.Sleep(time.Second * 5)

		// if the context is canceled, we stop
		select {
		case <-ctx.Done():
			outChan <- errors.New("waiting for pod to clear was aborted by context cancellation")
			return outChan
		default:
		}

		// fetch the pod by name
		_, err := podClient.Get(ext.PodName, metav1.GetOptions{})

		// if we got a "not found" message, then we are done.  This is the happy path.
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				ext.log("all pods cleared")
				outChan <- nil
				return outChan
			}

			// if the error was anything else, we return it upstream
			outChan <- err
			break
		}

	}

	return outChan
}

// waitForPodExit returns a channel that notifies when the checker pod exits
func (ext *Checker) waitForPodExit() chan error {

	// make the output channel we will return
	outChan := make(chan error, 2)

	// setup a pod watching client for our current KH pod
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)
	watcher, err := podClient.Watch(metav1.ListOptions{
		LabelSelector: kuberhealthyRunIDLabel + "=" + ext.currentCheckUUID,
	})
	defer watcher.Stop()

	// return the watch error as a channel if found
	if err != nil {
		outChan <- err
		return outChan
	}

	// watch events and return when the pod is in state running
	for e := range watcher.ResultChan() {

		// try to cast the incoming object to a pod and skip the event if we cant
		p, ok := e.Object.(*apiv1.Pod)
		if !ok {
			continue
		}

		// log.Debugln("Got event while watching for pod to stop:", e)

		// make sure the pod coming through the event channel has the right check uuid label
		ext.log("pod state is now:", string(p.Status.Phase))
		// read the status of this pod (its ours) and return if its succeeded or failed
		if p.Status.Phase == apiv1.PodSucceeded || p.Status.Phase == apiv1.PodFailed {
			outChan <- nil
			return outChan
		}

		// if the context is done, we break the loop and return
		select {
		case <-ext.ctx.Done():
			ext.log("external checker pod exit watch aborted due to check context being aborted")
			outChan <- nil
			return outChan
		default:
			// context is not canceled yet, continue
		}
	}

	outChan <- errors.New("waitForPodExit watch ended unexpectedly without finding a pod state")
	return outChan
}

// waitForPodRunning returns a channel that notifies when the checker pod is running
func (ext *Checker) waitForPodRunning() chan error {

	// make the output channel we will return
	outChan := make(chan error, 2)

	// setup a pod watching client for our current KH pod
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)

	// watch over and over again until we see our event or run out of time
	for {

		// if the context is done, we break the loop and return
		select {
		case <-ext.ctx.Done():
			ext.log("external checker pod startup watch aborted due to check context being aborted")
			outChan <- nil
			return outChan
		default:
			// context is not canceled yet, continue
		}

		// start watching
		watcher, err := podClient.Watch(metav1.ListOptions{
			LabelSelector: kuberhealthyRunIDLabel + "=" + ext.currentCheckUUID,
		})
		// cleanup watcher nicely

		// return the watch error as a channel if found
		if err != nil {
			outChan <- err
			watcher.Stop()
			return outChan
		}

		// watch events and return when the pod is in state running
		for e := range watcher.ResultChan() {

			// try to cast the incoming object to a pod and skip the event if we cant
			p, ok := e.Object.(*apiv1.Pod)
			if !ok {
				continue
			}

			// TODO - catch when the pod has an error image pull and return it as an error #201

			// read the status of this pod (its ours)
			ext.log("pod state is now:", string(p.Status.Phase))
			if p.Status.Phase == apiv1.PodRunning || p.Status.Phase == apiv1.PodFailed || p.Status.Phase == apiv1.PodSucceeded {
				outChan <- nil
				watcher.Stop()
				return outChan
			}

			// if the context is done, we break the loop and return
			select {
			case <-ext.ctx.Done():
				ext.log("external checker pod startup watch aborted due to check context being aborted")
				outChan <- nil
				watcher.Stop()
				return outChan
			default:
				// context is not canceled yet, continue
			}
		}
	}
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
func (ext *Checker) createPod() (*apiv1.Pod, error) {
	p := &apiv1.Pod{}
	p.Namespace = ext.Namespace
	p.Name = ext.PodName
	p.Annotations = ext.ExtraAnnotations
	p.Labels = ext.ExtraLabels
	ext.log("Creating external checker pod named", p.Name)
	p.Spec = ext.PodSpec
	ext.addKuberhealthyLabels(p)
	return ext.KubeClient.CoreV1().Pods(ext.Namespace).Create(p)
}

// configureUserPodSpec configures a user-specified pod spec with
// the unique and required fields for compatibility with an external
// kuberhealthy check.  Required environment variables and settings
// overwrite user-specified values.
func (ext *Checker) configureUserPodSpec() error {

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
	}

	// apply overwrite env vars on every container in the pod
	for i := range ext.PodSpec.Containers {
		ext.PodSpec.Containers[i].Env = resetInjectedContainerEnvVars(ext.PodSpec.Containers[i].Env, []string{KHReportingURL, KHRunUUID})
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

	// stack the kuberhealthy run id on top of the existing labels
	existingLabels := pod.ObjectMeta.Labels
	existingLabels[kuberhealthyRunIDLabel] = ext.currentCheckUUID
	existingLabels[kuberhealthyCheckNameLabel] = ext.CheckName
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

// podExists fetches the pod for the checker from the api server
// and returns a bool indicating if it exists or not
func (ext *Checker) podExists() (bool, error) {

	// setup a pod watching client for our current KH pod
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)
	p, err := podClient.Get(ext.PodName, metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}

	// if the pod has a start time that isn't zero, it exists
	if !p.Status.StartTime.IsZero() && p.Status.Phase != apiv1.PodFailed {
		return true, nil
	}

	return false, nil
}

// waitForShutdown waits for the external pod to shut down
func (ext *Checker) waitForShutdown(ctx context.Context) error {
	// repeatedly fetch the pod until its gone or the context
	// is canceled
	for {
		time.Sleep(time.Second * 5)
		exists, err := ext.podExists()
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}

		// see if the context has expired yet and give up if so
		select {
		case <-ctx.Done():
			return errors.New("timed out when waiting for pod to shutdown")
		default:
		}
	}
}

// Shutdown signals the checker to begin a shutdown and cleanup
func (ext *Checker) Shutdown() error {

	// cancel the context for this checker run
	ext.cancel()

	// make a context to track pod removal and cleanup
	ctx := context.Background()
	ctx, cancelCtx := context.WithCancel(ctx)

	// cancel the shutdown context after the timeout
	go func() {
		<-time.After(ext.Timeout())
		cancelCtx()
	}()

	// if the pod is deployed, delete it
	if ext.podDeployed() {
		err := ext.deletePod()
		if err != nil {
			ext.log("Error deleting pod during shutdown:", err.Error())
			return err
		}
		err = ext.waitForShutdown(ctx)
		if err != nil {
			ext.log("Error waiting for pod removal during shutdown:", err.Error())
			return err
		}
	}

	ext.log(ext.Name(), "Pod "+ext.PodName+" successfully shutdown.")
	return nil
}

// clearErrors clears all errors from the checker
func (ext *Checker) clearErrors() {
	ext.ErrorMessages = []string{}
}

// setError sets the error message for the checker and overwrites all prior state
func (ext *Checker) setError(s string) {
	ext.ErrorMessages = []string{s}
}

// addError adds an error to the errors list
func (ext *Checker) addError(s string) {
	ext.ErrorMessages = append(ext.ErrorMessages, s)
}

// podDeployed returns a bool indicating that the pod
// for this check exists and is deployed
func (ext *Checker) podDeployed() bool {
	ext.PodDeployedMu.Lock()
	defer ext.PodDeployedMu.Unlock()
	return ext.PodDeployed
}

// setPodDeployed sets the pod deployed state
func (ext *Checker) setPodDeployed(status bool) {
	ext.PodDeployedMu.Lock()
	defer ext.PodDeployedMu.Unlock()
	ext.PodDeployed = status
}

// getPodClient returns a client for Kubernetes pods
func (ext *Checker) getPodClient() typedv1.PodInterface {
	return ext.KubeClient.CoreV1().Pods(ext.Namespace)
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
