// Package podRestarts implements a checking tool for pods that are
// restarting too much.

// TODO - implement kuberhealthy check Interface

package podRestarts // import "github.com/Comcast/kuberhealthy/pkg/checks/podRestarts"

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const maxFailuresAllowed = 5

// Checker represents a long running pod restart checker.
type Checker struct {
	RestartObservations map[string][]RestartCountObservation
	Errors              []string
	Namespace           string
	MaxFailuresAllowed  int
	client              *kubernetes.Clientset
}

// RestartCountObservation keeps track of the number of restarts for a given pod
type RestartCountObservation struct {
	Time  time.Time
	Count int32
}

// New creates a new pod restart checker for a specific namespace, ready to use.
func New(namespace string) *Checker {
	return &Checker{
		RestartObservations: make(map[string][]RestartCountObservation),
		Errors:              []string{},
		Namespace:           namespace,
		MaxFailuresAllowed:  maxFailuresAllowed,
	}
}

// Name returns the name of this checker
func (prc *Checker) Name() string {
	return fmt.Sprintf("PodRestartChecker namespace %s", prc.Namespace)
}

// CheckNamespace returns the namespace of this checker
func (prc *Checker) CheckNamespace() string {
	return prc.Namespace
}

// Interval returns the interval at which this check runs
func (prc *Checker) Interval() time.Duration {
	return time.Minute * 5
}

// Timeout returns the maximum run time for this check before it times out
func (prc *Checker) Timeout() time.Duration {
	return time.Minute * 3
}

// CurrentStatus returns the status of the check as of right now
func (prc *Checker) CurrentStatus() (bool, []string) {
	if len(prc.Errors) > 0 {
		return false, prc.Errors
	}
	return true, prc.Errors
}

// Run implements the entrypoint for check execution
func (prc *Checker) Run(client *kubernetes.Clientset) error {
	doneChan := make(chan error)

	prc.client = client

	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := prc.doChecks()
		doneChan <- err
	}(doneChan)

	// wait for either a timeout or job completion
	select {
	case <-time.After(prc.Interval()):
		// The check has timed out because its time to run again
		// TODO - set check to failed because of timeout
		return errors.New("Failed to complete checks for " + prc.Name() + " in time!  Next run came up but check was still running.")
	case <-time.After(prc.Timeout()):
		// The check has timed out after its specified timeout period
		return errors.New("Failed to complete checks for " + prc.Name() + " in time!  Timeout was reached.")
	case err := <-doneChan:
		return err
	}
}

// Shutdown is implemented to satsify the KuberhealthyCheck interface, but
// no action is necessary.
func (prc *Checker) Shutdown() error {
	return nil
}

// clearErrors clears all errors
func (prc *Checker) clearErrors() {
	prc.Errors = []string{}
}

// UpdatePodRestartCheckCount adds new data to PodRestartCheck
func (p *RestartCountObservation) UpdatePodRestartCheckCount(r int32) {
	p.Time = time.Now()
	p.Count = r
}

// doChecks runs pod restart checks and returns a slice of found errors
// along with an indication of the check running correctly (error)
func (prc *Checker) doChecks() error {

	// create a list of pods in kube-system namespace
	l, err := prc.client.CoreV1().Pods(prc.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	// reap entries older than 1 hour from our restartObservations.  Also,
	// if a pod no longer exists, drop it from the status window.
	// TODO - make time window configurable, not just count
	prc.reapPodRestartChecks(l)

	// iterate through the list of pods and create a PodRestartCheck
	// struct to hold info about it
	for _, i := range l.Items {

		var restartMapItem RestartCountObservation
		s := i.Status.ContainerStatuses
		for _, i := range s {
			restartMapItem.UpdatePodRestartCheckCount(i.RestartCount)
		}

		// then put the struct into our meta map restartObservations
		n := i.ObjectMeta.Name
		prc.RestartObservations[n] = append(prc.RestartObservations[n], restartMapItem)
	}

	// evaluate the number of restarts per pod
	prc.Errors = prc.IdentifyRestartProblems()

	return nil
}

// ReapPodRestartChecks reaps old data from PodRestartCheck samplings
func (prc *Checker) reapPodRestartChecks(currentPods *v1.PodList) {

	for podName, restartObservations := range prc.RestartObservations {
		// if the pod no longer exists, then delete its observations
		if !podInPodList(podName, currentPods) {
			delete(prc.RestartObservations, podName)
			continue
		}
		var s []RestartCountObservation
		for _, observation := range restartObservations {
			// delete observations that are too old to matter
			// TODO: Take time range as an input rather than hard coding to an hour
			if observation.Time.After(time.Now().Add(-time.Hour)) {
				s = append(s, observation)
			}
		}
		prc.RestartObservations[podName] = s
	}
	for podName, restartObservations := range prc.RestartObservations {
		var mostRecentObservation RestartCountObservation
		var secondMostRecentObservation RestartCountObservation
		// Find the most recent PRC observation
		for _, observation := range restartObservations {
			if observation.Time.After(mostRecentObservation.Time) {
				mostRecentObservation = observation
			}

		}
		// Find the second most recent PRC observation
		for _, observation := range restartObservations {
			if observation.Time.Before(mostRecentObservation.Time) && observation.Time.After(secondMostRecentObservation.Time) {
				secondMostRecentObservation = observation
			}
		}
		// If the most recent observation restart count is less than the previous observation restart count, delete all the
		// entries that arent the most recent one, assuming that the pod was restarted but still has the same name.  This is a new
		// pod with a reset restart counter but has the same exact name as the previous pod that existed but was deleted
		// we see this in stateful sets
		if mostRecentObservation.Count < secondMostRecentObservation.Count {
			// delete all the observations
			delete(prc.RestartObservations, podName)
			//Add back in the most recent observation
			prc.RestartObservations[podName] = append(prc.RestartObservations[podName], mostRecentObservation)
		}
	}
}

// podInPodList determines if the specified pod is in a listing of pods
func podInPodList(podName string, l *v1.PodList) bool {
	for _, p := range l.Items {
		if p.Name == podName {
			return true
		}
	}
	return false
}

// IdentifyRestartProblems identifies pods that are restarting too much
// and returns a slice of string errors describing the issue
func (prc *Checker) IdentifyRestartProblems() []string {

	podRestartErrors := []string{}

	for i, p := range prc.RestartObservations {
		var min int32 = 2147483647 // set this to the max int32 value so when we assign it later, non 0 restart counts can be assigned
		var max int32
		// each restart observation check is evaluated to find the highest
		// and lowest count of restarts among all samplings of this pod
		for _, c := range p {
			if c.Count < min {
				min = c.Count
			}
			if c.Count > max {
				max = c.Count
			}
		}
		if (max - min) > int32(prc.MaxFailuresAllowed) {
			errorMessage := prc.Namespace + " pod restarts for pod " + i + " greater than " + strconv.Itoa(int(prc.MaxFailuresAllowed)) + " in the last hour."
			podRestartErrors = append(podRestartErrors, errorMessage)
		}
	}

	return podRestartErrors
}
