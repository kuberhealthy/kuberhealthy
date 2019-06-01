// Package external is a kuberhealthy checker that acts as an operator
// to run external images as checks.
package external

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/prometheus/common/log"
	"k8s.io/client-go/kubernetes"
	"errors"

	apiv1 "k8s.io/api/core/v1"
)

// NamePrefix is the name of this kuberhealthy checker
var NamePrefix = "external-check"

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
	CheckName          string // the name of this checker
	Namespace     string
	ErrorMessages []string
	Image         string        // the docker image URL to spin up
	RunInterval   time.Duration // how often this check runs a loop
	maxRunTime       time.Duration // time check must run completely within
	kubeClient    *kubernetes.Clientset
	PodSpec       *apiv1.PodSpec // the user-provided spec of the pod
	PodDeployed   bool           // indicates the pod exists in the API
	PodDeployedMu sync.Mutex
	PodName       string // the name of the deployed pod
	RunID         string // the uuid of the current run
}

// New creates a new Checker object
func New() (*Checker, error) {

	hostname := getHostname()
	var tolerations []apiv1.Toleration

	testDS := Checker{
		ErrorMessages: []string{},
		Namespace:     namespace,
		CheckName:          DefaultName,
		RunInterval:   defaultRunInterval,
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
// the RunInterval and is executed by the kuberhealthy checker
func (ext *Checker) Run(client *kubernetes.Clientset) error {
	// TODO
	// TODO
	// TODO
	// TODO
	// TODO
	// TODO

	// validate all inputs for the checker

	// if the pod spec is unspecified, we return an error
	if ext.PodSpec == nil {
		return errors.New("Unable to determine pod spec for cheker.  Pod spec was nil.")
	}

// if containers are not set, then we return an error
	if len(ext.PodSpec.Containers) == 0 && len(ext.PodSpec.InitContainers) == 0 {
		return errors.New("No containers found in check's PodSpec.")
		// TODO - dump detected spec?
	}

	// ensure that at least one container is defined
	if len(ext.PodSpec.Containers) == 0 {
		return errors.New("No containers found in check's PodSpec.")
	}

	// ensure that all containers have an image set
	for _, c := range ext.PodSpec.Containers {
		if len(c.Image) == 0 {
		return errors.New("No image found in check's PodSpec for container " + c.Name+".")
		}
	}



	// create kubernetes job from user's pod spec
	job := ext.configureUserPodSpec()

	// determine check pod's run UUID and set whitelist in CRD for this check

	// Spawn kubernetes pod (not job)

	// watch for pod to start with a timeout (include time for a new node to be created)

	// watch for pod to run with a timeout

	// watch for pod to complete

	// TODO - can we make pods not restart after crashing?

		return nil
}

// configureUserPodSpec configures a user-specified pod spec with
// the unique and required fields for compatibility with an external
// kuberhealthy check.

// TODO
func (ext *Checker) configureUserPodSpec() (error) {

	// set ovrerrides like env var for pod name and env var for where to send responses to
	// Set environment variable for run UUID
	// wrap pod spec in job spec

	// specify environment variables that need applied
	overwriteEnvVars := []apiv1.EnvVar{
	apiv1.EnvVar{
		Name: "KUBERHEALTHY_URL",
		Value: "", // TODO
	},
	apiv1.EnvVar{
		Name: "KUBERHEALTHY_CHECK_RUN_ID",
		Value: "", // TODO
	},
	}


	// apply overwrite env vars on every container in the pod
	for i := range ext.PodSpec.Containers {
		ext.PodSpec.Containers[i].Env = append(ext.PodSpec.Containers[i].Env,overwriteEnvVars...)
	}
		
	return nil
		
}

// fetchPod fetches the pod for the checker from the api server
// and returns a bool indicating if it exists or not
func (ext *Checker) fetchPod() (bool, error) {
	podClient := ext.GetClient()
	var firstQuery bool
	var more string
	// pagination
	for firstQuery || len(more) > 0 {
		firstQuery = false
		dsList, err := dsClient.List(metav1.ListOptions{
			Continue: more,
		})
		if err != nil {
			return false, err
		}
		more = dsList.Continue

		// check results for our daemonset
		for _, item := range dsList.Items {
			if item.GetName() == dsc.DaemonSetName {
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
		exists, err := dsc.fetchDS()
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
	}
}

// getPodClient returns a Pod client for kubernetes
func (ext *Checker) getPodClient() v1beta1.PodInterface {
	log.Debug("Creating Pod client.")
	return ext.client.ExtensionsV1beta1().Pods(ext.Namespace)
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
		ext.remove()
		ext.waitForPodRemoval(ctx)
	}

	log.Infoln(ext.Name(), "Pod "+ext.PodName+" ready for shutdown.")
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
