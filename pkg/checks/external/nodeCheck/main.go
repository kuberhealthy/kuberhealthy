package nodeCheck

import (
	"context"
	"errors"
	"net"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external"
)

// WaitForKuberhealthy waits for the the kuberhealthy endpoint (KH_REPORTING_URL) to be contactable by the checker pod
// on a given node
func WaitForKuberhealthy(ctx context.Context) {

	kuberhealthyEndpoint := os.Getenv(external.KHReportingURL)

	// check the length of the reporting url to make sure we pulled one properly
	if len(kuberhealthyEndpoint) < 1 {
		// Just log the error and return since this check will not work if there's an error getting the kuberhealthyEndpoint
		log.Errorln("Error getting kuberhealthy reporting URL from environment variable", external.KHReportingURL, "was blank")
		return
	}

	log.Infoln("Checking if the kuberhealthy endpoint:", kuberhealthyEndpoint, "is ready.")
	select {
	case <-waitForKuberhealthyEndpointReady(kuberhealthyEndpoint):
		log.Infoln("Kuberhealthy endpoint:", kuberhealthyEndpoint, "is ready. Proceeding to run check.")
		return
	case <-time.After(time.Minute):
		log.Errorln("Timed out checking if kuberhealthy endpoint:", kuberhealthyEndpoint, "is ready. " +
			"Check may or may not complete successfully.")
	case <-ctx.Done():
		log.Errorln("Context cancelled waiting for Kuberhealthy endpoint to be ready.")
	}
}

// WaitForKubeProxy waits for kube proxy to be ready and running on the node before running the check. Assumes that the
// kube-proxy pod follows the naming convention: "kube-proxy-{nodeName}"
func WaitForKubeProxy(client *kubernetes.Clientset, namespace string, ctx context.Context) {

	khPod, err := getKHPod(client, namespace)
	if err != nil {
		// Just log the error and return since this check will not work if there's an error getting the podName
		log.Errorln("Error getting kuberhealthy pod:", err)
		return
	}
	log.Infoln("Pod is on node:", khPod.Spec.NodeName)
	log.Infoln("Checking if kube-proxy is running and ready on this node:", khPod.Spec.NodeName)
	select {
	case err := <- waitForKubeProxyReady(client, khPod.Spec.NodeName):
		if err != nil {
			// Just log the error.
			log.Errorln(err)
		}
		log.Infoln("Kube proxy is ready. Proceeding to run check.")
	case <- time.After(time.Duration(time.Minute)):
		// Just log the error.
		log.Errorln("Timed out checking if kube proxy is ready. Check node:", khPod.Spec.NodeName,
		"Check may or may not complete successfully.")
	case <-ctx.Done():
		log.Errorln("Context cancelled waiting for kube proxy to be ready and running")
	}
}

// WaitForNodeAge checks the node's age to see if its less than the minimum node age. If so, sleeps for a given sleep duration.
func WaitForNodeAge(client *kubernetes.Clientset, namespace string, minNodeAge time.Duration, sleepDuration time.Duration, ctx context.Context) {

	khPod, err := getKHPod(client, namespace)
	if err != nil {
		// Just log the error and return since this check will not work if there's an error getting the podName
		log.Errorln("Error getting kuberhealthy pod:", err)
		return
	}
	log.Infoln("Pod is on node:", khPod.Spec.NodeName)

	nodeNew, err := checkNodeNew(client, khPod, minNodeAge)
	if err != nil {
		// Just log the error and return since this check will not work if there's an error getting the node
		log.Errorln("Error checking if node is new:", khPod.Spec.NodeName, err)
		return
	}

	if nodeNew {
		log.Infoln("Node is less than", minNodeAge, "old. Sleeping for", sleepDuration)
		select {
		case <- ctx.Done():
			log.Errorln("Context cancelled while sleeping for:", sleepDuration)
		default:
			time.Sleep(sleepDuration)
			log.Infoln("Done sleeping. Proceeding to run check")
		}
	}
}

// getKHPod gets the kuberhealthy pod currently running. The pod is needed to get the pod's node information.
func getKHPod(client *kubernetes.Clientset, namespace string) (*corev1.Pod, error) {

	var khPod *corev1.Pod
	podName, err := os.Hostname()
	if err != nil {
		// Just log the error and return since this check will not work if there's an error getting the podName
		log.Errorln("Error getting hostname:", err)
		return khPod, err
	}

	log.Infoln("Getting pod:", podName, "in order to get its node information")
	khPod, err = client.CoreV1().Pods(namespace).Get(podName, v1.GetOptions{})
	if err != nil {
		// Just log the error and return since this check will not work if there's an error getting the pod
		log.Errorln("Error getting pod:", err)
		return khPod, err
	}
	return khPod, err
}

// checkIfNodeIsNew checks the age of the node the kuberhealthy check pod is on to determine if its "new" or not.
func checkNodeNew(client *kubernetes.Clientset, khPod *corev1.Pod, minNodeAge time.Duration) (bool, error) {

	var newNode bool
	node, err := client.CoreV1().Nodes().Get(khPod.Spec.NodeName, v1.GetOptions{})
	if err != nil {
		// Just log the error and return since this check will not work if there's an error getting the node
		log.Errorln("Failed to get node:", khPod.Spec.NodeName, err)
		return newNode, err
	}

	// get current age of the node
	nodeAge := time.Now().Sub(node.CreationTimestamp.Time)
	// if the node the pod is on is less than 3 minutes old, wait 1 minute before running check.
	log.Infoln("Check running on node: ", node.Name, "with node age:", nodeAge)

	if nodeAge < minNodeAge {
		newNode = true
		return newNode, nil
	}

	return newNode, nil
}

// waitForKuberhealthyEndpointReady hits the kuberhealthy endpoint every 3 seconds to see if the node is ready to reach
// the endpoint.
func waitForKuberhealthyEndpointReady(kuberhealthyEndpoint string) chan struct{} {

	doneChan := make(chan struct{}, 1)

	for {
		_, err := net.LookupHost(kuberhealthyEndpoint)
		if err == nil {
			log.Infoln(kuberhealthyEndpoint, "is ready.")
			doneChan <- struct{}{}
			return doneChan
		} else {
			log.Infoln(kuberhealthyEndpoint, "is not ready yet..." + err.Error())
		}
		time.Sleep(time.Second * 3)
	}
}

// waitForKubeProxyReady fetches the kube proxy pod every 5 seconds until it's ready and running.
func waitForKubeProxyReady(client *kubernetes.Clientset, nodeName string) chan error {

	kubeProxyName := "kube-proxy-" + nodeName
	log.Infoln("Waiting for kube-proxy pod to be ready:", kubeProxyName, "on node:", nodeName)
	doneChan := make(chan error, 1)

	for {
		kubeProxyReady, err := checkKubeProxyPod(client, kubeProxyName)
		if err != nil {
			log.Errorln("Error getting kube proxy pod:", err)
			doneChan <- err
			return doneChan
		}

		if kubeProxyReady {
			log.Infoln("Kube proxy: ", kubeProxyName, "is ready!")
			return doneChan
		}
		time.Sleep(time.Second * 5)
	}
}

// checkKubeProxyPod gets the kube proxy pod and checks if its ready and running.
func checkKubeProxyPod(client *kubernetes.Clientset, podName string) (bool, error) {

	var kubeProxyReady bool

	kubeProxyPod, err := client.CoreV1().Pods("").Get(podName, v1.GetOptions{})
	if err != nil {
		errorMessage := "Failed to get kube-proxy pod: " + podName + ". Error: " + err.Error()
		log.Errorln(errorMessage)
		return kubeProxyReady, errors.New(errorMessage)
	}

	var podReady = true
	for _, cs := range kubeProxyPod.Status.Conditions {
		if cs.Type != corev1.PodReady {
			podReady = false
			break
		}
	}

	if kubeProxyPod.Status.Phase == corev1.PodRunning && podReady {
		log.Infoln(kubeProxyPod.Name, "is in status running and ready.")
		kubeProxyReady = true
		return kubeProxyReady, nil
	}

	log.Infoln(kubeProxyPod.Name, "is in status:", kubeProxyPod.Status.Phase, "and ready: ",
		podReady)
	return kubeProxyReady, nil
}
