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

// Package podRestarts implements a checking tool for pods that are
// restarting too much.

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	checkclient "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
)

const defaultMaxFailuresAllowed = 10

var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var Namespace string
var CheckTimeout time.Duration
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
	if len(Namespace) == 0 {
		log.Errorln("ERROR: The POD_NAMESPACE environment variable has not been set.")
		return
	}

	// Grab and verify environment variables and set them as global vars
	checkTimeout := os.Getenv("CHECK_POD_TIMEOUT")
	if len(checkTimeout) == 0 {
		log.Errorln("ERROR: The CHECK_TIMEOUT environment variable has not been set.")
		return
	}

	var err error
	CheckTimeout, err = time.ParseDuration(checkTimeout)
	if err != nil {
		log.Errorln("Error parsing timeout for check", checkTimeout, err)
		return
	}

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
	log.Infoln("Running Pod Restarts checker")
	doneChan := make(chan error)

	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := prc.doChecks()
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
			for _, msg := range prc.BadPods {
				errorMessages = append(errorMessages, msg)
			}
			return reportKHFailure(errorMessages)

		} else {
			return reportKHSuccess()
		}
	}
}

// doChecks grabs all events in a given namespace, then checks for pods with event type "Warning" with reason "BackOff",
// and an event count greater than the MaxFailuresAllowed. If any of these pods are found, an error message is appended
// to Checker struct errorMessages.
func (prc *Checker) doChecks() error {

	log.Infoln("Checking for pod BackOff events for all pods in the namespace:", prc.Namespace)

	podWarningEvents, err := prc.client.CoreV1().Events(prc.Namespace).List(metav1.ListOptions{FieldSelector: "type=Warning"})
	if err != nil {
		return err
	}

	if len(podWarningEvents.Items) != 0 {
		log.Infoln("Found `Warning` events in the namespace:", prc.Namespace)

		for _, event := range podWarningEvents.Items {

			// Checks for pods with BackOff events greater than the MaxFailuresAllowed
			if event.InvolvedObject.Kind == "Pod" && event.Reason == "BackOff" && event.Count > prc.MaxFailuresAllowed {
				errorMessage := "Found: " + strconv.FormatInt(int64(event.Count), 10) + " `BackOff` events for pod: " + event.InvolvedObject.Name + " in namespace: " + prc.Namespace

				log.Infoln(errorMessage)

				prc.BadPods[event.InvolvedObject.Name] = errorMessage
			}
		}
	}

	for pod := range prc.BadPods {
		err := prc.verifyBadPodRestartExists(pod)
		if err != nil {
			return err
		}
	}

	return err
}

// verifyBadPodRestartExists removes the bad pod found from the events list if the pod no longer exists
func (prc *Checker) verifyBadPodRestartExists(podName string) error {

	_, err := prc.client.CoreV1().Pods(prc.Namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
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
