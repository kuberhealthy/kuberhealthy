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
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	checkclient "github.com/Comcast/kuberhealthy/pkg/checks/external/checkClient"
	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
)

const defaultMaxFailuresAllowed = 5

var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var Namespace string
var RunWindow time.Duration
var BadPodRestarts sync.Map
var MaxFailuresAllowed int

// Checker represents a long running pod restart checker.
type Checker struct {
	RestartObservations map[string]map[string]int32
	Namespace           string
	MaxFailuresAllowed  int
	client              *kubernetes.Clientset
	errorMessages       []string
}

func init() {

	// Grab and verify environment variables and set them as global vars
	Namespace = os.Getenv("POD_NAMESPACE")
	if len(Namespace) == 0 {
		log.Errorln("ERROR: The POD_NAMESPACE environment variable has not been set.")
		return
	}

	runWindow := os.Getenv("CHECK_RUN_WINDOW")
	if len(runWindow) == 0 {
		log.Errorln("ERROR: The CHECK_RUN_WINDOW environment variable has not been set.")
		return
	}

	var err error
	RunWindow, err = time.ParseDuration(runWindow)
	if err != nil {
		log.Errorln("Error parsing run window for check", runWindow, err)
		return
	}

	MaxFailuresAllowed = defaultMaxFailuresAllowed
	maxFailuresAllowed := os.Getenv("MAX_FAILURES_ALLOWED")
	if len(maxFailuresAllowed) != 0 {
		MaxFailuresAllowed, err = strconv.Atoi(maxFailuresAllowed)
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

	// Create new pod restarts checker
	prc := New()

	// Add created client to the pod restarts checker
	prc.client = client

	log.Infoln("Enabling pod restarts checker")

	// Populate the RestartObservations map for all current pods found with their restart count
	err = prc.configurePodRestartCount()
	if err != nil {
		log.Errorln("Failed to configure pods with restart count observations, error:", err)
	}

	// Run the pod restarts check status reporter in the background
	go prc.statusReporter()

	// Run shutdown after the check run time window in the background
	go shutdownAfterDuration(RunWindow)

	// Run the check to watch for pod changes
	prc.watchForPodChanges()
}

// New creates a new pod restart checker for a specific namespace, ready to use.
func New() *Checker {
	return &Checker{
		RestartObservations: make(map[string]map[string]int32),
		Namespace:           Namespace,
		MaxFailuresAllowed:  MaxFailuresAllowed,
		errorMessages:       []string{},
	}
}

// shutdownAfterDuration shuts down the program after the run time window
func shutdownAfterDuration(duration time.Duration) {
	time.Sleep(duration)

	log.Infoln("Check run has completed successfully. Reporting check success to Kuberhealthy.")

	length := 0
	BadPodRestarts.Range(func(_, _ interface{}) bool {
		length++
		return true
	})

	if length == 0 {
		err := checkclient.ReportSuccess()
		if err != nil {
			log.Println("Error reporting success to Kuberhealthy servers:", err)
		}
		log.Println("Successfully reported success to Kuberhealthy servers")
	}

	os.Exit(0)
}

// configurePodRestartCount populates the RestartObservations map for all pods with their container restart counts
func (prc *Checker) configurePodRestartCount() error {

	log.Infoln("Configuring pod restart observations for all pods in namespace:", prc.Namespace)

	l, err := prc.client.CoreV1().Pods(prc.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, p := range l.Items {
		prc.addPodRestartCount(p)
	}

	return err
}

// watchForPodChanges spawns a watcher to catch various pod events within a namespace.
// If pods are added, the pods restart count is added to the checkers RestartObservation map
// If pods are modified, we check for bad pod restarts
// If pods are deleted, we remove the pod from the checkers RestartObservation map and the BadPodRestarts map
func (prc *Checker) watchForPodChanges() {

	for {
		log.Infoln("Spawned watcher for pod changes in namespace:", prc.Namespace)

		// wait a second so we don't retry too quickly on error
		time.Sleep(time.Second)
		podClient := prc.client.CoreV1().Pods(prc.Namespace)

		watcher, err := podClient.Watch(metav1.ListOptions{})
		if err != nil {
			log.Errorln("error watching pods", err)
			watcher.Stop()
		}

		for pod := range watcher.ResultChan() {
			switch pod.Type {
			case watch.Added:
				log.Debugln("pod restart monitor saw an added event, adding pod restart count")
				pod, ok := pod.Object.(*v1.Pod)
				if !ok {
					continue
				}
				prc.addPodRestartCount(*pod)
			case watch.Modified:
				log.Debugln("pod restart monitor saw a modified event")
				pod, ok := pod.Object.(*v1.Pod)
				if !ok {
					continue
				}
				prc.checkBadPodRestarts(*pod)
			case watch.Deleted:
				log.Debugln("pod restart monitor saw a deleted event")
				pod, ok := pod.Object.(*v1.Pod)
				if ok {
					delete(prc.RestartObservations, pod.Name)
					prc.removeBadPodRestarts(pod.Name)
				}
			case watch.Bookmark:
				log.Debugln("pod restart monitor saw a bookmark event and ignored it")
			case watch.Error:
				log.Debugln("pod restart monitor saw an error event")
				e := pod.Object.(*metav1.Status)
				log.Errorln("Error when watching for pod restart changes:", e.Reason)
				break
			default:
				log.Warningln("pod restart monitor saw an unknown event type and ignored it:", pod.Type)
			}
		}
		watcher.Stop()
	}
}

// addPodRestartCount adds new pod restart count to the RestartObservations map
func (prc *Checker) addPodRestartCount(pod v1.Pod) {
	for _, s := range pod.Status.ContainerStatuses {
		containerMap := make(map[string]int32)
		containerMap[s.Name] = s.RestartCount
		prc.RestartObservations[pod.Name] = containerMap
	}
}

// checkBadPodRestarts calculates the delta in pod restarts and checks for too many restarts
func (prc *Checker) checkBadPodRestarts(pod v1.Pod) {

	for _, s := range pod.Status.ContainerStatuses {
		oldRestartCount := prc.RestartObservations[pod.Name][s.Name]
		newRestartCount := s.RestartCount

		if newRestartCount-oldRestartCount > 5 {
			BadPodRestarts.Store(pod.Name, newRestartCount)
		}
	}
}

// removeBadPodRestarts removes the pod from the BadPodRestart map if the pod has been deleted
func (prc *Checker) removeBadPodRestarts(podName string) {

	BadPodRestarts.Range(func(_, _ interface{}) bool {
		_, exists := BadPodRestarts.Load(podName)
		if exists {
			log.Infoln("Pod:", podName, "with too many restarts has been deleted. Removing from status report.")
			BadPodRestarts.Delete(podName)
		}
		return true
	})
}

// statusReporter sends status reports to Kuberhealthy every minute on bad pod restarts
func (prc *Checker) statusReporter() {
	var err error

	log.Infoln("Starting status reporter to send status reports to Kuberhealthy if bad pod restarts are found")

	// Start ticker for sending reports to Kuberhealthy
	ticker := time.NewTicker(1 * time.Minute)

	for {
		<-ticker.C

		length := 0
		BadPodRestarts.Range(func(_, _ interface{}) bool {
			length++
			return true
		})

		if length > 0 {
			log.Println("Bad pod with restarts found, reporting check failure to Kuberhealthy servers")

			prc.errorMessages = []string{} // clear error messages before every failure report

			BadPodRestarts.Range(func(k, v interface{}) bool {

				errorMessage := "Pod restarts for pod: " + k.(string) + " is greater than " + strconv.Itoa(int(prc.MaxFailuresAllowed)) + " in the last hour. Restart Count is at: " + strconv.Itoa(int(v.(int32)))
				prc.errorMessages = append(prc.errorMessages, errorMessage)

				return true
			})
			err = checkclient.ReportFailure(prc.errorMessages)
			if err != nil {
				log.Println("Error reporting failure to Kuberhealthy servers:", err)
			}
			log.Println("Reported failure to Kuberhealthy servers")

			continue
		}

		log.Println("No bad pod with restarts found, starting next tick")
	}
}
