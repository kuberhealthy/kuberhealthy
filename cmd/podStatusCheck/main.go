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
	checkclient.Debug = true
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
		log.Infoln("Amount of failures found: ", len(failures))
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

// testing with a switch
// in the case where we have a pod status of "Running" but has a container error in "Crashloopbackoff"
// needs to be further researched, as this specific scenario may always be caught by the podRestarts check
// are there any other conditions that we need to look for where a pod is Running or Succeeded but the
// containers within them are unhealthy???
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

		// find pods that are in phase Running/Succeeded
		// find pods that are in phase Pending/Failed/Unknown then add to list failed pods
		// log if there is no match to the 5 possible pod status phases
		switch {
		case pod.Status.Phase == v1.PodRunning:
			continue
		case pod.Status.Phase == v1.PodSucceeded:
			continue
		case pod.Status.Phase == v1.PodPending:
			failures = append(failures, pod.Name+" is in pod status phase "+string(pod.Status.Phase)+" ")
		case pod.Status.Phase == v1.PodFailed:
			failures = append(failures, pod.Name+" is in pod status phase "+string(pod.Status.Phase)+" ")
		case pod.Status.Phase == v1.PodUnknown:
			failures = append(failures, pod.Name+" is in pod status phase "+string(pod.Status.Phase)+" ")
		default:
			log.Info(pod.Name + " is not in one of the five possible pod status phases " + string(pod.Status.Phase) + " ")
		}
	}

	return failures, nil

}
