// Package podStatus implements a pod health checker for Kuberhealthy.  Pods are checked
// to ensure they are not restarting too much and are in a healthy lifecycle
// phase.
package podStatus // import "github.com/Comcast/kuberhealthy/pkg/checks/podStatus"

import (
	"errors"
	"fmt"
	"time"

	// required for oidc kubectl testing
	log "github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Checker validates that pods within a namespace are in a healthy state
type Checker struct {
	FailureTimeStamp map[string]time.Time
	Errors           []string
	Namespace        string
	MaxTimeInFailure float64 // TODO - make configurable
	client           *kubernetes.Clientset
}

// New returns a new Checker
func New(namespace string) *Checker {
	return &Checker{
		Namespace:        namespace,
		FailureTimeStamp: make(map[string]time.Time),
		MaxTimeInFailure: 300,
		Errors:           []string{},
	}
}

// Name returns the name of this checker
func (psc *Checker) Name() string {
	return fmt.Sprintf("PodStatusChecker namespace %s", psc.Namespace)
}

// CheckNamespace returns the namespace of this checker
func (psc *Checker) CheckNamespace() string {
	return psc.Namespace
}

// Interval returns the interval at which this check runs
func (psc *Checker) Interval() time.Duration {
	return time.Minute * 2
}

// Timeout returns the maximum run time for this check before it times out
func (psc *Checker) Timeout() time.Duration {
	return time.Minute * 1
}

// Shutdown is implemented to satisfy KuberhealthyCheck but is not used
func (psc *Checker) Shutdown() error {
	return nil
}

// CurrentStatus returns the status of the check as of right now
func (psc *Checker) CurrentStatus() (bool, []string) {
	if len(psc.Errors) > 0 {
		return false, psc.Errors
	}
	return true, psc.Errors
}

// clearErrors clears all errors
func (psc *Checker) clearErrors() {
	psc.Errors = []string{}
}

// Run implements the entrypoint for check execution
func (psc *Checker) Run(client *kubernetes.Clientset) error {
	doneChan := make(chan error)

	psc.client = client
	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := psc.doChecks()
		doneChan <- err
	}(doneChan)

	// wait for either a timeout or job completion
	select {
	case <-time.After(psc.Interval()):
		// The check has timed out because its time to run again
		// TODO - set check to failed because of timeout
		return errors.New("Failed to complete checks for " + psc.Name() + " in time!  Next run came up but check was still running.")
	case <-time.After(psc.Timeout()):
		// The check has timed out after its specified timeout period
		return errors.New("Failed to complete checks for " + psc.Name() + " in time!  Timeout was reached.")
	case err := <-doneChan:
		return err
	}
}

// doChecks does validations on pod status and returns an error if one is encountered
// while communicating with the API. Errors from pods are set directly and
// only system errors are returned
func (psc *Checker) doChecks() error {

	// get the status of all pods
	podStatus, err := psc.podFailures()
	if err != nil {
		return err
	}

	// if errors found, set them in the status
	if len(podStatus) > 0 {
		var newErrorSet []string
		for _, p := range podStatus {
			log.Errorln(psc.Name(), "Error found when checking pods: "+p)
			newErrorSet = append(newErrorSet, p)
		}
		psc.Errors = newErrorSet
		return nil
	}

	psc.clearErrors()
	return nil
}

// podFailures goes through kube-system or a specified namespace and determines the pod health
// failures is a list of pods that are having issues.
func (psc *Checker) podFailures() (failures []string, err error) {
	pods, err := psc.client.CoreV1().Pods(psc.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return
	}
	// process failures and manage failed containers in psc.FailureTimeStamp
	for _, pod := range pods.Items {

		// dont check pod health if the pod is less than 10 minutes old
		if time.Now().Sub(pod.CreationTimestamp.Time).Minutes() < 10 {
			continue
		}

		for _, container := range pod.Status.ContainerStatuses {
			currentlyFailedContainer := pod.Name + " ( " + container.Name + " ) "
			if container.Ready {
				delete(psc.FailureTimeStamp, currentlyFailedContainer)
				continue
			}
			timestamp, exists := psc.FailureTimeStamp[currentlyFailedContainer]
			// add newly failed containers to the FailureTimeStamp map
			if !exists {
				psc.FailureTimeStamp[currentlyFailedContainer] = time.Now()
				continue
			}
			// if a container has been failed for x time, alert
			if time.Now().Sub(timestamp).Seconds() > psc.MaxTimeInFailure {
				failures = append(failures, currentlyFailedContainer)
			}
		}
	}

	// loop through all containers in the failed container map and
	// remove any that do not exist anymore
	for previouslyFailedContainer := range psc.FailureTimeStamp {
		purge := true
		for _, pod := range pods.Items {
			for _, container := range pod.Status.ContainerStatuses {
				currentlyFailedContainer := pod.Name + " ( " + container.Name + " )"
				if currentlyFailedContainer == previouslyFailedContainer {
					purge = false
					break
				}
			}
			if !purge {
				break
			}
		}
		if purge {
			delete(psc.FailureTimeStamp, previouslyFailedContainer)
		}
	}
	return
}

// componentFailures goes through all the components of the system and determines their health
func componentFailures(client kubernetes.Interface) (failures []string, err error) {
	componentList, err := client.CoreV1().ComponentStatuses().List(metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, component := range componentList.Items {
		for _, condition := range component.Conditions {
			if condition.Status != v1.ConditionTrue {
				failures = append(failures, component.Name)
			}
		}
	}
	return
}
