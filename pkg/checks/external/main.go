// Package external is a kuberhealthy checker that acts as an operator
// to run external images as checks.
package external

import (
	"context"
	"errors"
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

// defaultMaxRunTime is the default time a pod is allowed to run when this checker is created
const defaultMaxRunTime = time.Minute * 15

// defaultMaxStartTime is the default time that a pod is required to start within
const defaultMaxStartTime = time.Minute * 5

// DefaultName is used when no check name is supplied
var DefaultName = "external-check"

// kubeConfigFile is the default location to check for a kubernetes configuration file
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// constants for using the kuberhealthy check CRD
const checkCRDGroup = "comcast.github.io"
const checkCRDVersion = "v1"
const checkCRDResource = "khchecks"
const stateCRDResource = "khstates"
const stateCRDGroup = "comcast.github.io"
const stateCRDVersion = "v1"

// Checker implements a KuberhealthyCheck for external
// check execution and lifecycle management.
type Checker struct {
	CheckName                string // the name of this checker
	Namespace                string
	ErrorMessages            []string
	RunInterval              time.Duration // how often this check runs a loop
	maxRunTime               time.Duration // time check must run completely within after switching to 'Running'
	startupTimeout           time.Duration // the time an external checker pod has to become 'Running' after starting
	KubeClient               *kubernetes.Clientset
	PodSpec                  apiv1.PodSpec // the current pod spec we are using after enforcement of settings
	OriginalPodSpec          apiv1.PodSpec // the user-provided spec of the pod
	PodDeployed              bool          // indicates the pod exists in the API
	PodDeployedMu            sync.Mutex
	PodName                  string             // the name of the deployed pod
	RunID                    string             // the uuid of the current run
	KuberhealthyReportingURL string             // the URL that the check should want to report results back to
	currentCheckUUID         string             // the UUID of the current external checker running
	Debug                    bool               // indicates we should run in debug mode - run once and stop
	cancelFunc               context.CancelFunc // used to cancel things in-flight
	ctx                      context.Context    // a context used for tracking check runs
}

// New creates a new external checker
func New(client *kubernetes.Clientset, checkConfig *khcheckcrd.KuberhealthyCheck, reportingURL string) *Checker {
	if len(checkConfig.Namespace) == 0 {
		checkConfig.Namespace = "kuberhealthy"
	}

	log.Debugf("Creating external check from check config: %+v \n", checkConfig)

	// build the checker object
	return &Checker{
		ErrorMessages:            []string{},
		Namespace:                checkConfig.Namespace,
		CheckName:                checkConfig.Name,
		KuberhealthyReportingURL: reportingURL,
		maxRunTime:               defaultMaxRunTime,
		startupTimeout:           defaultMaxStartTime,
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
	return ext.maxRunTime
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

	var defaultCheck khcheckcrd.KuberhealthyCheck

	// make a new crd check client
	checkClient, err := ext.NewCheckClient(ext.Namespace)
	if err != nil {
		return &defaultCheck, err
	}

	// get the item in question and return it along with any errors
	log.Debugln("Fetching check", ext.CheckName, "in namespace", ext.Namespace)
	checkConfig, err := checkClient.Get(metav1.GetOptions{}, checkCRDResource, ext.Namespace, ext.CheckName)
	if err != nil {
		return &defaultCheck, err
	}

	return checkConfig, err
}

// NewCheckClient creates a new client for khcheckcrd resources
func (ext *Checker) NewCheckClient(namespace string) (*khcheckcrd.KuberhealthyCheckClient, error) {
	// make a new crd check client
	return khcheckcrd.Client(checkCRDGroup, checkCRDVersion, kubeConfigFile, namespace)
}

// setUUID sets the current whitelisted UUID for the checker and updates it on the server
func (ext *Checker) setUUID(uuid string) error {

	log.Debugln("Set expected UUID to:", uuid)
	checkConfig, err := ext.getCheck()
	if err != nil {
		if !strings.Contains(err.Error(), "could not find the requested resource") {
			return err
		}
	}

	// update the check config and write it back to the struct
	checkConfig.Spec.CurrentUUID = uuid
	checkConfig.ObjectMeta.Namespace = ext.Namespace

	// make a new crd check client
	checkClient, err := ext.NewCheckClient(ext.Namespace)
	if err != nil {
		return err
	}

	// update the resource with the new values we want
	_, err = checkClient.Update(checkConfig, checkCRDResource, ext.Namespace, ext.Name())
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
		return errors.New("failed to clean up pods before starting external checker: " + err.Error())
	}

	// waiting for all checker pods are gone...
	err = <-ext.waitForAllPodsToClear(ext.ctx)
	if err != nil {
		return errors.New("failed while waiting for pods to clean up: " + err.Error())
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
	case <-time.After(ext.startupTimeout):
		ext.cancel() // cancel the watch context, we have timed out
		errorMessage := "failed to see pod running within timeout"
		err := ext.deletePod()
		if err != nil {
			errorMessage = errorMessage + " and an error occurred when deleting the pod:" + err.Error()
		}
		return errors.New(errorMessage)
	case <-ext.waitForPodRunning():
		ext.log("External check pod is running:", ext.PodName)
	}

	// flag the pod as running until this run ends
	ext.setPodDeployed(true)
	defer ext.setPodDeployed(false)

	// validate that the pod was able to update its khstate
	timeoutChan := time.After(ext.maxRunTime)
	ext.log("Waiting for pod status to be reported from pod", ext.PodName, "in namespace", ext.Namespace)
	select {
	case <-timeoutChan:
		ext.log("Timed out waiting for checker pod to report in")
		ext.cancel() // cancel the watch context, we have timed out
		err := ext.deletePod()
		errorMessage := "pod status callback was not seen before timeout"
		if err != nil {
			errorMessage = errorMessage + " and an error occurred when deleting the pod:" + err.Error()
		}
		return errors.New(errorMessage)
	case err = <-ext.waitForPodStatusUpdate(lastReportTime, ext.ctx):
		if err != nil {
			ext.log("External checker had an error waiting for pod status to update:", err.Error())
			return err
		}
		ext.log("External check pod has reported status for this check iteration:", ext.PodName)
	}

	// validate that the pod stopped running properly
	select {
	case <-timeoutChan:
		ext.log("Timed out waiting for pod to stop running:", ext.PodName)
		ext.cancel() // cancel the watch context, we have timed out
		err := ext.deletePod()
		errorMessage := "pod ran too long and was shut down"
		if err != nil {
			errorMessage = errorMessage + " and an error occurred when deleting the pod:" + err.Error()
		}
		return errors.New(errorMessage)
	case err = <-ext.waitForPodExit(ext.ctx):
		if err != nil {
			ext.log("External checker had an error while waiting for pod to exit:", err.Error())
			return err
		}
		ext.log("External check pod is done running:", ext.PodName)
	}

	ext.log("Run completed!")

	return nil
}

// Log writes a normal InfoLn message output prefixed with this checker's name on it
func (ext *Checker) log(s ...string) {
	log.Infoln(ext.CheckName+":", s)
}

// stopPod stops any pods running because of this external checker
func (ext *Checker) deletePod() error {
	ext.log("Deleting all checker pods")
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)
	return podClient.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: kuberhealthyCheckNameLabel + "=" + ext.CheckName,
	})
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

// NewKHStateClient creates a new client for khstate resources
func (ext *Checker) NewKHStateClient(namespace string) (*khstatecrd.KuberhealthyStateClient, error) {
	// make a new crd check client
	return khstatecrd.Client(stateCRDGroup, stateCRDVersion, kubeConfigFile, namespace)
}

// getCheckLastUpdateTime fetches the last time the khstate custom resource for this check was updated
// as a time.Tiem.
func (ext *Checker) getCheckLastUpdateTime(ctx context.Context) (time.Time, error) {

	// setup a client for watching our status update
	stateClient, err := ext.NewKHStateClient(ext.Namespace)
	if err != nil {
		return time.Time{}, err
	}

	// fetch the khstate as it exists
	khstate, err := stateClient.Get(metav1.GetOptions{}, stateCRDResource, ext.CheckName)
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

	// make the output channel we will return and close it whenever we are done
	outChan := make(chan error, 2)
	defer close(outChan)

	// watch events and return when the pod is in state running
	for {

		// wait half a second between requests
		time.Sleep(time.Second / 2)
		log.Debugln("Waiting for external checker pod to report in...")

		// if the context is canceled, we stop
		select {
		case <-ctx.Done():
			outChan <- errors.New("waiting for pod to clear was aborted by context cancellation")
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

	return outChan
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

		// if the context is canceled, we stop
		select {
		case <-ctx.Done():
			outChan <- errors.New("waiting for pod to clear was aborted by context cancellation")
		default:
		}

		// wait half a second between requests
		time.Sleep(time.Second / 2)

		// fetch the pod by name
		_, err := podClient.Get(ext.PodName, metav1.GetOptions{})

		// if we got a "not found" message, then we are done.  This is the happy path.
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return outChan
			}

			// if the error was anything else, we return it upstream
			outChan <- err
			break
		}

		log.Debugln("Waiting for checker pod", ext.PodName, "to clear...")
	}

	return outChan
}

// waitForPodExit returns a channel that notifies when the checker pod exits
func (ext *Checker) waitForPodExit(ctx context.Context) chan error {

	// make the output channel we will return
	outChan := make(chan error, 2)

	// setup a pod watching client for our current KH pod
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)
	watcher, err := podClient.Watch(metav1.ListOptions{
		LabelSelector: kuberhealthyRunIDLabel + "=" + ext.currentCheckUUID,
	})

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
		if p.Labels[kuberhealthyRunIDLabel] != ext.currentCheckUUID {
			continue
		}

		// read the status of this pod (its ours) and return if its succeeded or failed
		if p.Status.Phase == apiv1.PodSucceeded || p.Status.Phase == apiv1.PodFailed {
			outChan <- nil
			return outChan
		}

		// if the context is done, we break the loop and return
		select {
		case <-ctx.Done():
			outChan <- errors.New("external checker pod completion watch aborted")
			return outChan
		default:
			// context is not canceled yet, continue
		}
	}

	outChan <- errors.New("external checker watch aborted pre-maturely")
	return outChan
}

// waitForPodRunning returns a channel that notifies when the checker pod is running
func (ext *Checker) waitForPodRunning() chan error {

	// make the output channel we will return
	outChan := make(chan error, 2)

	// setup a pod watching client for our current KH pod
	podClient := ext.KubeClient.CoreV1().Pods(ext.Namespace)
	watcher, err := podClient.Watch(metav1.ListOptions{
		LabelSelector: kuberhealthyRunIDLabel + "=" + ext.currentCheckUUID,
	})

	// return the watch error as a channel if found
	if err != nil {
		outChan <- err
		return outChan
	}

	// watch events and return when the pod is in state running
	for e := range watcher.ResultChan() {

		// log.Debugln("Got event while watching for pod to start running:", e)

		// try to cast the incoming object to a pod and skip the event if we cant
		p, ok := e.Object.(*apiv1.Pod)
		if !ok {
			continue
		}

		// make sure the pod coming through the event channel has the right check uuid label
		if p.Labels[kuberhealthyRunIDLabel] != ext.currentCheckUUID {
			continue
		}

		// read the status of this pod (its ours)
		if p.Status.Phase == apiv1.PodRunning || p.Status.Phase == apiv1.PodFailed {
			outChan <- nil
			return outChan
		}

		// if the context is done, we break the loop and return
		select {
		case <-ext.ctx.Done():
			outChan <- errors.New("external checker pod startup watch aborted")
			return outChan
		default:
			// context is not canceled yet, continue
		}
	}

	outChan <- errors.New("external checker watch aborted pre-maturely")
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
func (ext *Checker) createPod() (*apiv1.Pod, error) {
	p := &apiv1.Pod{}
	p.Namespace = ext.Namespace
	p.Name = ext.PodName
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
		time.Sleep(time.Second)
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
