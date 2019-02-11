// Package componentStatus implements a componentstatus checker.
package componentStatus // import "github.com/Comcast/kuberhealthy/pkg/checks/componentStatus"

import (
	"errors"
	"time"

	// required for oidc kubectl testing
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
}

// Checker validates componentstatus objects within the cluster.
type Checker struct {
	Errors           []string
	FailureTimeStamp map[string]time.Time
	MaxTimeInFailure float64 // TODO - make configurable
	client           *kubernetes.Clientset
}

// New returns a new Checker
func New() *Checker {
	return &Checker{
		FailureTimeStamp: make(map[string]time.Time),
		MaxTimeInFailure: 300,
		Errors:           []string{},
	}
}

// Name returns the name of this checker
func (csc *Checker) Name() string {
	return "ComponentStatusChecker"
}

// CheckNamespace returns the namespace of this checker
func (csc *Checker) CheckNamespace() string {
	return ""
}

// Interval returns the interval at which this check runs
func (csc *Checker) Interval() time.Duration {
	return time.Minute * 2
}

// Timeout returns the maximum run time for this check before it times out
func (csc *Checker) Timeout() time.Duration {
	return time.Minute * 1
}

// Shutdown is implemented to satisfy the KuberhealthyCheck interface, but
// no action is necessary.
func (csc *Checker) Shutdown() error {
	return nil
}

// CurrentStatus returns the status of the check as of right now
func (csc *Checker) CurrentStatus() (bool, []string) {
	if len(csc.Errors) > 0 {
		return false, csc.Errors
	}
	return true, csc.Errors
}

// clearErrors clears all errors
func (csc *Checker) clearErrors() {
	csc.Errors = []string{}
}

// Run implements the entrypoint for check execution
func (csc *Checker) Run(client *kubernetes.Clientset) error {
	doneChan := make(chan error)
	csc.client = client
	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := csc.doChecks()
		doneChan <- err
	}(doneChan)

	// wait for either a timeout or job completion
	select {
	case <-time.After(csc.Interval()):
		// The check has timed out because its time to run again
		return errors.New("Failed to complete checks for " + csc.Name() + " in time!  Next run came up but check was still running.")
	case <-time.After(csc.Timeout()):
		// The check has timed out after its specified timeout period
		return errors.New("Failed to complete checks for " + csc.Name() + " in time!  Timeout was reached.")
	case err := <-doneChan:
		return err
	}
}

// doChecks executes checks.
func (csc *Checker) doChecks() error {

	// list componentstatuses
	componentList, err := csc.client.CoreV1().ComponentStatuses().List(metav1.ListOptions{})
	if err != nil {
		csc.Errors = []string{"Error creating client when checking componentstatuses: " + err.Error()}
		return nil
	}

	// check componentstatuses
	var errorMessages []string
	for _, component := range componentList.Items {
		for _, condition := range component.Conditions {
			currentComponent := component.Name
			// remove the condition from the map if its not still in a failure state
			if condition.Status == v1.ConditionTrue {
				delete(csc.FailureTimeStamp, currentComponent)
				continue
			}
			// if unhealthy...
			timestamp, exists := csc.FailureTimeStamp[currentComponent]
			// add newly failed components to the FailureTimeStamp map
			if !exists {
				csc.FailureTimeStamp[currentComponent] = time.Now()
				continue
			}
			// if a container has been failed for X time, alert
			if time.Now().Sub(timestamp).Seconds() > csc.MaxTimeInFailure {
				errorMessages = append(errorMessages, "componentstatus "+component.Name+" is in a bad state: "+string(condition.Status))
			}

			// loop through all conditions in the failed condition map and
			// remove any that do not exist anymore
			for previouslyFailedComponent := range csc.FailureTimeStamp {
				for _, component := range componentList.Items {
					currentComponent := component.Name
					if currentComponent == previouslyFailedComponent {
						break
					}
				}
				delete(csc.FailureTimeStamp, previouslyFailedComponent)
			}
		}
	}

	// if errors found, set them, if not, clear all
	if len(errorMessages) > 0 {
		csc.Errors = errorMessages
	} else {
		csc.clearErrors()
	}

	// nil indicates no system error occurred when checking
	return nil
}
