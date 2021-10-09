// Package podRestarts implements a checking tool for pods that are
// restarting too much.

package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	checkclient "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
)

const defaultMaxFailuresAllowed = 10
const defaultCheckTimeout = 10 * time.Minute

// KubeConfigFile is a variable containing file path of Kubernetes config files
var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// Namespace is a variable to allow code to target all namespaces or a single namespace
var Namespace string

// CheckTimeout is a variable for how long code should run before it should retry.
var CheckTimeout time.Duration

// MaxFailuresAllowed is a variable for how many times the pod should retry before stopping.
var MaxFailuresAllowed int32

// Checker represents a long running pod restart checker.
type Checker struct {
	Namespace          string
	MaxFailuresAllowed int32
	BadPods            map[string]string
	client             *kubernetes.Clientset
}

func init() {
	// Grab and verify environment variables and set them as global vars
	Namespace = os.Getenv("POD_NAMESPACE")
	if Namespace == "" {
		log.Infoln("Looking for pods across all namespaces, this requires a cluster role")
		// it is the same value but we are being explicit that we are listing pods in all namespaces
		Namespace = v1.NamespaceAll
	} else {
		log.Infoln("Looking for pods in namespace:", Namespace)
	}

	// Set check time limit to default
	CheckTimeout = defaultCheckTimeout
	// Get the deadline time in unix from the env var
	timeDeadline, err := checkclient.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}
	CheckTimeout = timeDeadline.Sub(time.Now().Add(time.Second * 5))
	log.Infoln("Check time limit set to:", CheckTimeout)

	MaxFailuresAllowed = defaultMaxFailuresAllowed
	maxFailuresAllowed := os.Getenv("MAX_FAILURES_ALLOWED")
	if len(maxFailuresAllowed) != 0 {
		conversion, err := strconv.ParseInt(maxFailuresAllowed, 10, 32)
		MaxFailuresAllowed = int32(conversion)
		if err != nil {
			log.Errorln("Error converting maxFailuresAllowed:", maxFailuresAllowed, "to int, err:", err)
			return
		}
	}
}

func main() {

	// Create client
	client, err := kubeClient.Create(KubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	// Create new pod restarts checker with Kubernetes client
	prc := New(client)

	// Run check
	err = prc.Run()
	if err != nil {
		log.Errorln("Error running Pod Restarts check:", err)
		os.Exit(2)
	}
	log.Infoln("Done running Pod Restarts check")
	os.Exit(0)
}

// New creates a new pod restart checker for a specific namespace, ready to use.
func New(client *kubernetes.Clientset) *Checker {
	return &Checker{
		Namespace:          Namespace,
		MaxFailuresAllowed: MaxFailuresAllowed,
		BadPods:            make(map[string]string),
		client:             client,
	}
}

// Run starts the go routine to run checks, reports whether or not the check completely successfully, and finally checks
// for any errors in the Checker struct and re
func (prc *Checker) Run() error {
	// TODO: refactor function to receive context on exported function in next breaking change.
	ctx := context.TODO()

	log.Infoln("Running Pod Restarts checker")
	doneChan := make(chan error)

	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := prc.doChecks(ctx)
		doneChan <- err
	}(doneChan)

	// wait for either a timeout or job completion
	select {
	case <-time.After(CheckTimeout):
		// The check has timed out after its specified timeout period
		errorMessage := "Failed to complete Pod Restart check in time! Timeout was reached."
		err := reportKHFailure([]string{errorMessage})
		if err != nil {
			return err
		}
		return err
	case err := <-doneChan:
		if len(prc.BadPods) != 0 || err != nil {
			var errorMessages []string
			if err != nil {
				log.Error(err)
				errorMessages = append(errorMessages, err.Error())
			}
			for _, msg := range prc.BadPods {
				errorMessages = append(errorMessages, msg)
			}
			return reportKHFailure(errorMessages)

		}
		return reportKHSuccess()
	}
}

// doChecks grabs all events in a given namespace, then checks for pods with event type "Warning" with reason "BackOff",
// and an event count greater than the MaxFailuresAllowed. If any of these pods are found, an error message is appended
// to Checker struct errorMessages.
func (prc *Checker) doChecks(ctx context.Context) error {

	log.Infoln("Checking for pod BackOff events for all pods in the namespace:", prc.Namespace)

	podWarningEvents, err := prc.client.CoreV1().Events(prc.Namespace).List(ctx, metav1.ListOptions{FieldSelector: "type=Warning"})
	if err != nil {
		return err
	}

	if len(podWarningEvents.Items) != 0 {
		log.Infoln("Found `Warning` events in the namespace:", prc.Namespace)

		for _, event := range podWarningEvents.Items {

			// Checks for pods with BackOff events greater than the MaxFailuresAllowed
			if event.InvolvedObject.Kind == "Pod" && event.Reason == "BackOff" && event.Count > prc.MaxFailuresAllowed {
				errorMessage := "Found: " + strconv.FormatInt(int64(event.Count), 10) + " `BackOff` events for pod: " + event.InvolvedObject.Name + " in namespace: " + event.Namespace

				log.Infoln(errorMessage)

				// We could be checking for pods in all namespaces so prefix the namespace
				prc.BadPods[event.InvolvedObject.Namespace+"/"+event.InvolvedObject.Name] = errorMessage
			}
		}
	}

	for pod := range prc.BadPods {
		err := prc.verifyBadPodRestartExists(ctx, pod)
		if err != nil {
			return err
		}
	}
	return err
}

// verifyBadPodRestartExists removes the bad pod found from the events list if the pod no longer exists
func (prc *Checker) verifyBadPodRestartExists(ctx context.Context, pod string) error {

	// Pod is in the form namespace/pod_name
	parts := strings.Split(pod, "/")
	namespace := parts[0]
	podName := parts[1]

	_, err := prc.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found") {
			log.Infoln("Bad Pod:", podName, "no longer exists. Removing from bad pods map")
			delete(prc.BadPods, podName)
		} else {
			log.Infoln("Error getting bad pod:", podName, err)
			return err
		}
	}
	return nil
}

// reportKHSuccess reports success to Kuberhealthy servers and verifies the report successfully went through
func reportKHSuccess() error {
	err := checkclient.ReportSuccess()
	if err != nil {
		log.Println("Error reporting success to Kuberhealthy servers:", err)
		return err
	}
	log.Println("Successfully reported success to Kuberhealthy servers")
	return err
}

// reportKHFailure reports failure to Kuberhealthy servers and verifies the report successfully went through
func reportKHFailure(errorMessages []string) error {
	err := checkclient.ReportFailure(errorMessages)
	if err != nil {
		log.Println("Error reporting failure to Kuberhealthy servers:", err)
		return err
	}
	log.Println("Successfully reported failure to Kuberhealthy servers")
	return err
}
