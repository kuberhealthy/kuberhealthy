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
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	checkclient "github.com/Comcast/kuberhealthy/pkg/checks/external/checkClient"
	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
)

const defaultMaxFailuresAllowed = 10

var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var Namespace string
var MaxFailuresAllowed int

// Checker represents a long running pod restart checker.
type Checker struct {
	//RestartObservations map[string]map[string]int32
	Namespace           string
	//MaxFailuresAllowed  int
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

	// Create new pod restarts checker with Kubernetes client
	prc := New(client)

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
func New(client *kubernetes.Clientset) *Checker {
	return &Checker{
		Namespace:           Namespace,
		errorMessages:       []string{},
		client: 			 client,
	}
}


//
//label := ""
//for k := range pod.GetLabels() {
//label = k
//break
//}
//watch, err := clientset.CoreV1().Pods(namespace).Watch(metav1.ListOptions{
//LabelSelector: label,
//})
//if err != nil {
//log.Fatal(err.Error())
//}
//go func() {
//	for event := range watch.ResultChan() {
//		fmt.Printf("Type: %v\n", event.Type)
//		p, ok := event.Object.(*v1.Pod)
//		if !ok {
//			log.Fatal("unexpected type")
//		}
//		fmt.Println(p.Status.ContainerStatuses)
//		fmt.Println(p.Status.Phase)
//	}
//}()
//time.Sleep(5 * time.Second)


func (prc *Checker) runCheck() error {

	log.Infoln("Checking for pod BackOff events for all pods in the namespace:", prc.Namespace)

	podWarningEvents, err := prc.client.CoreV1().Events(prc.Namespace).List(metav1.ListOptions{FieldSelector: "type=Warning"})
	if err != nil {
		return err
	}

	for _, event := range podWarningEvents.Items {

		if event.InvolvedObject.Kind == "pod" && event.Reason == "BackOff" && event.Count > {

			log.Infoln("Found BackOff events for pod:", event.InvolvedObject.Name, "in namespace:", prc.Namespace)

		}
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

	//Ranging through BadPodRestarts map for deleted pod. If found, remove bad pod from map.
	BadPodRestarts.Range(func(_, _ interface{}) bool {
		_, exists := BadPodRestarts.Load(podName)
		if exists {
			log.Infoln("Pod:", podName, "with too many restarts has been deleted. Removing from status report.")
			BadPodRestarts.Delete(podName)
		}
		return true
	})

	length := 0

	BadPodRestarts.Range(func(_, _ interface{}) bool {
		length++
		return true
	})

	if length == 0 {
		log.Infoln("No more bad pod restarts found. Changing report status to OK.")
		err := checkclient.ReportSuccess()
		if err != nil {
			log.Println("Error reporting success to Kuberhealthy servers:", err)
		}
		log.Println("Successfully reported success to Kuberhealthy servers")
	}
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
