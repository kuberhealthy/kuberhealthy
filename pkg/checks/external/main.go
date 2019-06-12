// Package external is a kuberhealthy checker that acts as an operator
// to run external images as checks.
package external

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/prometheus/common/log"
	"k8s.io/client-go/kubernetes"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// DefaultKuberhealthyReportingURL is the default location that external checks
// are expected to report into.
const DefaultKuberhealthyReportingURL = "http://kuberhealthy.kuberhealthy.svc.local"

// NamePrefix is the name of this kuberhealthy checker
var NamePrefix = DefaultName

// DefaultName is used when no check name is supplied
var DefaultName = "external-check"

// namespace indicates the namespace of the kuberhealthy
// pod that is running this check
var namespace = os.Getenv("POD_NAMESPACE")

// defaultRunInterval is the default time we assume this check
// should run on unless specified
var defaultRunInterval = time.Minute * 10

// Checker implements a KuberhealthyCheck for external
// check execution and lifecycle management.
type Checker struct {
	CheckName                string // the name of this checker
	Namespace                string
	ErrorMessages            []string
	Image                    string        // the docker image URL to spin up
	RunInterval              time.Duration // how often this check runs a loop
	maxRunTime               time.Duration // time check must run completely within
	kubeClient               *kubernetes.Clientset
	PodSpec                  *apiv1.PodSpec // the user-provided spec of the pod
	PodDeployed              bool           // indicates the pod exists in the API
	PodDeployedMu            sync.Mutex
	PodName                  string             // the name of the deployed pod
	RunID                    string             // the uuid of the current run
	KuberhealthyReportingURL string             // the URL that the check should want to report results back to
}

// New creates a new external checker
func New() (*Checker, error) {

	testDS := Checker{
		ErrorMessages:            []string{},
		Namespace:                namespace,
		CheckName:                DefaultName,
		RunInterval:              defaultRunInterval,
		KuberhealthyReportingURL: DefaultKuberhealthyReportingURL,
	}

	return &testDS, nil
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
	return NamePrefix + "-" + ext.CheckName
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
	// TODO
	// TODO
	// TODO
	// TODO
	// TODO
	// TODO

	// if the pod spec is unspecified, we return an error
	if ext.PodSpec == nil {
		return errors.New("unable to determine pod spec for checker  Pod spec was nil")
	}

	// if containers are not set, then we return an error
	if len(ext.PodSpec.Containers) == 0 && len(ext.PodSpec.InitContainers) == 0 {
		return errors.New("no containers found in checks PodSpec")
		// TODO - dump detected spec?
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

	// create kubernetes job from user's pod spec
	err := ext.configureUserPodSpec()
	if err != nil {
		return errors.New("failed to configure pod spec for Kubernetes from user specified pod spec: " + err.Error())
	}

	// Spawn kubernetes pod (not job)
	log.Infoln("Creating pod:", ext.CheckName)
	createdPod, err := ext.createPod()
	if err != nil {
		return errors.New("failed to create pod for checker: " + err.Error())
	}

	log.Infoln("Created pod",createdPod.Name,"in namespace",createdPod.Namespace)

	// watch for pod to start with a timeout (include time for a new node to be created)

	select {
	case <-time.After(time.Minute * 10):
		return errors.New("failed to create pod for checker after 10 minute timeout")
	}


	// watch for pod to run with a timeout

	// watch for pod to complete

	// TODO - can we make pods not restart after crashing?

	return nil
}

// waitForPodRunning returns a channel that notifies when the specified pod name is running
func (ext *Checker) waitForPodRunning(podName string, namespace string) chan struct{}{
	podClient := ext.kubeClient.CoreV1().Pods(namespace).Watch(metav1.ListOptions{
		// TODO - watch for pod by unique run label
	})
}

// createPod creates the checker pod using the kubernetes API
func (ext *Checker) createPod() (*apiv1.Pod, error) {
	p := &apiv1.Pod{}
	p.Namespace = ext.Namespace
	p.Name = ext.PodName
	p.Spec = *ext.PodSpec
	_, err := ext.kubeClient.CoreV1().Pods(ext.Namespace).Create(p)
	return err
}

// configureUserPodSpec configures a user-specified pod spec with
// the unique and required fields for compatibility with an external
// kuberhealthy check.

// configureUserPodSpec takes in the user's pod spec as seen by this checker and
// adds in kuberhealthy settings such as the required environment variables.
func (ext *Checker) configureUserPodSpec() error {

	// set overrides like env var for pod name and env var for where to send responses to
	// Set environment variable for run UUID
	// wrap pod spec in job spec

	// set the pod running the check's pod name
	ext.PodSpec.Hostname = ext.CheckName

	// specify environment variables that need applied.  We apply environment
	// variables that set the report-in URL of kuberhealthy along with
	// the unique run ID of this pod
	overwriteEnvVars := []apiv1.EnvVar{
		{
			Name:  "KUBERHEALTHY_URL",
			Value: ext.KuberhealthyReportingURL,
		},
		{
			Name:  "KUBERHEALTHY_RUN_ID",
			Value: ext.createCheckUUID(),
		},
	}

	// apply overwrite env vars on every container in the pod
	for i := range ext.PodSpec.Containers {
		ext.PodSpec.Containers[i].Env = append(ext.PodSpec.Containers[i].Env, overwriteEnvVars...)
	}

	// TODO - add unique run ID as a label

	return nil
}

// createCheckUUID creates a UUID that represents a single run of the external check
func (ext *Checker) createCheckUUID() string {
	uuid, err := uuid.FromString(ext.CheckName + time.Now().String())
	if err != nil {
		log.Errorln("External checker had error creating UUID for external check run:", err)
	}
	return uuid.String()
}

// fetchPod fetches the pod for the checker from the api server
// and returns a bool indicating if it exists or not
func (ext *Checker) fetchPod() (bool, error) {
	podClient := ext.getPodClient()
	var firstQuery bool
	var more string
	// pagination
	for firstQuery || len(more) > 0 {
		firstQuery = false
		podList, err := podClient.List(metav1.ListOptions{
			Continue: more,
		})
		if err != nil {
			return false, err
		}
		more = podList.Continue

		// check results for our pod
		for _, item := range podList.Items {
			if item.GetName() == ext.PodName {
				// ds does exist, return true
				return true, nil
			}
		}
	}

	// daemonset does not exist, return false
	return false, nil
}

// waitForShutdown waits for the external pod to shut down
func (ext *Checker) waitForShutdown(ctx context.Context) error {
	// repeatedly fetch the pod until its gone or the context
	// is canceled
	for {
		time.Sleep(time.Second / 2)
		exists, err := ext.fetchPod()
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
	}
}

// Shutdown signals the DS to begin a cleanup
func (ext *Checker) Shutdown() error {

	// make a context to satisfy pod removal
	ctx := context.Background()
	ctx, cancelCtx := context.WithCancel(ctx)

	// cancel the shutdown context after the timeout
	go func() {
		<-time.After(ext.Timeout())
		cancelCtx()
	}()

	// if the pod is deployed, delete it
	if ext.podDeployed() {
		ext.removePod()
		ext.waitForPodRemoval()
	}

	log.Infoln(ext.Name(), "Pod "+ext.PodName+" ready for shutdown.")
	return nil

}

// waitForPodRemoval waits for the external checker pod to be removed
func (ext *Checker) waitForPodRemoval() error {
	// TODO
	return nil
}

// removePod removes the external checker pod
func (ext *Checker) removePod() error {
	// TODO
	return nil
}

// clearErrors clears all errors from the checker
func (ext *Checker) clearErrors() {
	ext.ErrorMessages = []string{}
}

// podDeployed returns a bool indicating that the pod
// for this check exists and is deployedj
func (ext *Checker) podDeployed() bool {
	ext.PodDeployedMu.Lock()
	defer ext.PodDeployedMu.Unlock()
	return ext.PodDeployed
}

// setPodDeployedStatus sets the pod deployed state
func (ext *Checker) setPodDeployed(status bool) {
	ext.PodDeployedMu.Lock()
	defer ext.PodDeployedMu.Unlock()
	ext.PodDeployed = status
}

// getPodClient returns a client for Kubernetes pods
func (ext *Checker) getPodClient() typedv1.PodInterface {
	return ext.kubeClient.CoreV1().Pods(ext.Namespace)
}
