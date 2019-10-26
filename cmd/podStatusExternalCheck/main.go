// Package podStatus implements a pod health checker for Kuberhealthy.  Pods are checked
// to ensure they are not restarting too much and are in a healthy lifecycle
// phase.
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

	failures, err := findPodsNotRunning(client)
	if err != nil {
		err = checkclient.ReportFailure([]string{err.Error()})
		if err != nil {
			log.Println("Error", err)
			os.Exit(1)
		}
		return
	}
	if len(failures) >= 1 {
		err = checkclient.ReportFailure(failures)
		if err != nil {
			log.Println("Error reporting failures to Kuberhealthy servers", err)
			os.Exit(1)
		}
		return
	}
	err = checkclient.ReportSuccess()
	if err != nil {
		log.Println("Error reporting success to Kuberhealthy servers", err)
		os.Exit(1)
	}
}



func findPodsNotRunning(client *kubernetes.Clientset) ([]string, error) {
	var failures []string
	pods, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return failures, err
	}
	for _, pod := range pods.Items {

		// dont check pod health if the pod is less than 10 minutes old
		if time.Now().Sub(pod.CreationTimestamp.Time).Minutes() < 10 {
			continue
		}
		if pod.Status.Phase != v1.PodRunning {
			failures = append(failures, pod.Name + " has bad state " + string(pod.Status.Phase))
		}

	}
	return failures, nil

}
