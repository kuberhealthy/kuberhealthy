// Package podStatus implements a pod health checker for Kuberhealthy.  Pods are checked
// to ensure they are not restarting too much and are in a healthy lifecycle phase.
package main

import (
	checkclient "github.com/Comcast/kuberhealthy/pkg/checks/external/checkClient"
	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
	"os"
	"path/filepath"
	"time"

	// required for oidc kubectl testing
	log "github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var namespace string

func init() {
	namespace = os.Getenv("TARGET_NAMESPACE")
}

func main() {
	client, err := kubeClient.Create(KubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	// get our list of failed pods, if there are any errors, report failures to Kuberhealthy servers.
	failures, err := findPodsNotRunning(client)
	if err != nil {
		err = checkclient.ReportFailure([]string{err.Error()})
		if err != nil {
			log.Println("Error", err)
			os.Exit(1)
		}
		return
	}
	// report our list of failed pods to Kuberhealthy servers.
	if len(failures) >= 1 {
		err = checkclient.ReportFailure(failures)
		if err != nil {
			log.Println("Error reporting failures to Kuberhealthy servers", err)
			os.Exit(1)
		}
		return
	}
	// report success to Kuberhealthy servers if there were no failed pods in our list.
	err = checkclient.ReportSuccess()
	log.Infoln("Reporting Success, no unhealthy pods found.")
	if err != nil {
		log.Println("Error reporting success to Kuberhealthy servers", err)
		os.Exit(1)
	}
}

// testing a different way to find failed container statuses
// seen instances where Pods that are "Running" but are in "Crashloopbackoff" due to container "Waiting"
// need to hit "Running" statuses in both Pod and Container
// possible pod status phases
// Pending | Running | Succeeded | Failed | Unknown
// possible container status phases
// Waiting | Running | Terminated


func findPodsNotRunning(client *kubernetes.Clientset) ([]string, error) {

	var failures []string

	pods, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return failures, err
	}
    // start iteration over pods
	for _, pod := range pods.Items {
		// check if the pod age is over 10 minutes
		if time.Now().Sub(pod.CreationTimestamp.Time).Minutes() < 10 {
			continue
		}
		// filter out pods that are not Running
		if pod.Status.Phase != v1.PodRunning {
			log.Infoln("Found pod: ", pod.Name)
			failures = append(failures, pod.Name+" has bad state "+string(pod.Status.Phase))
			continue
		}
		// for the pods that are Running we start checking container statuses within them
		var containersNotReady bool
		for _, container := range pod.Status.ContainerStatuses {
			if !container.Ready	{
				log.Infoln("Found bad container: ", container.Name, "in", pod.Name)
				containersNotReady = true
				break
			}
		}
		if containersNotReady {
		    failures = append(failures, pod.Name+" has containers that are not ready.")
		}
	}
log.Infoln("Here are the list of failed pods: ", failures)

return failures, nil

}


/* findPodsNotRunning gets a list of pods that are older than 10 minutes and contain all Pod status phases
// that are NOT healthy/Running.
func findPodsNotRunning(client *kubernetes.Clientset) ([]string, error) {

	var failures []string

	log.Infoln("Finding pods not running in namespace: ", namespace)

	pods, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{})

	if err != nil {
		return failures, err
	}
	for _, pod := range pods.Items {

		log.Infoln("Checking pod: ", pod.Name)

		// dont check pod health if the pod is less than 10 minutes old.
		if time.Now().Sub(pod.CreationTimestamp.Time).Minutes() < 10 {
			continue
		}

		// debug
		log.Infoln("Pod: ", pod.Name+" has a status of ", pod.Status.Phase)

		// create a list of of unhealthy pods that were found.
		if pod.Status.Phase != v1.PodRunning {
			failures = append(failures, pod.Name+" has bad state "+string(pod.Status.Phase))
		}

	}
	return failures, nil

}
*/