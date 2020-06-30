package nodeCheck

import (
	"errors"
	"net"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/api/v1/pod"
)

// WaitForKuberhealthyEndpoint first checks the node's age to make sure it's not less than three minutes old. If so, it
// checks if the kuberhealthy endpoint (KH_REPORTING_URL) can be hit, determining whether or not the proper netfilter
// rules have been set up on the node in order to hit the kuberhealthy service.
func WaitForKuberhealthyEndpoint(client *kubernetes.Clientset, namespace string, kuberhealthyEndpoint string) {

	khPod, err := getKHPod(client, namespace)
	if err != nil {
		// Just log the error and return since this check will not work if there's an error getting the podName
		log.Errorln("Error getting kuberhealthy pod:", err)
		return
	}

	log.Infoln("Pod is on node:", khPod.Spec.NodeName)

	nodeMinAge := time.Minute * 3
	nodeNew, err := checkNodeNew(client, khPod, nodeMinAge)
	if err != nil {
		// Just log the error and return since this check will not work if there's an error getting the node
		log.Errorln("Error checking if node is new:", khPod.Spec.NodeName, err)
		return
	}

	if nodeNew {
		log.Infoln("Node is less than", nodeMinAge, "old. Checking if the kuberhealthy endpoint is ready.")
		select {
		case <-waitForKuberhealthyEndpointReady(kuberhealthyEndpoint):
			log.Infoln("Kuberhealthy endpoint:", kuberhealthyEndpoint, "is ready. Proceeding to run check.")
			return
		case <-time.After(time.Minute):
			log.Errorln("Time out checking if kuberhealthy endpoint:", kuberhealthyEndpoint, "is ready. " +
				"Check may or may not complete successfully.")
		}
	}
}

// WaitForKubeProxy first checks the node's age to make sure it's not less than three minutes old. If so, it sleep for
// one minute and checks if kube proxy is ready and running on the node before running the check.
func WaitForKubeProxy(client *kubernetes.Clientset, namespace string) {

	khPod, err := getKHPod(client, namespace)
	if err != nil {
		// Just log the error and return since this check will not work if there's an error getting the podName
		log.Errorln("Error getting kuberhealthy pod:", err)
		return
	}
	log.Infoln("Pod is on node:", khPod.Spec.NodeName)

	nodeMinAge := time.Minute * 3
	nodeNew, err := checkNodeNew(client, khPod, nodeMinAge)
	if err != nil {
		// Just log the error and return since this check will not work if there's an error getting the node
		log.Errorln("Error checking if node is new:", khPod.Spec.NodeName, err)
		return
	}

	if nodeNew {
		sleepDuration := time.Minute
		log.Infoln("Node is less than", nodeMinAge, "old. Sleeping for", sleepDuration)
		time.Sleep(sleepDuration)

		log.Infoln("Checking if kube-proxy is running and ready.")

		select {
		case err := <- waitForKubeProxyReady(client, khPod.Spec.NodeName):
			if err != nil {
				// Just log the error.
				log.Errorln(err)
			}
			log.Infoln("Kube proxy is ready. Proceeding to run check.")
		case <- time.After(time.Duration(time.Minute)):
			// Just log the error.
			// TO DO: figure out how to address this. Should the check to skip this run and pass up an error instead?
			// If kube-proxy isn't ready and running, there's definitely something wrong with the new node coming up.
			log.Errorln("Timed out checking if kube proxy is ready. Check node:", khPod.Spec.NodeName,
			"Check may or may not complete successfully.")
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
func checkNodeNew(client *kubernetes.Clientset, khPod *corev1.Pod, nodeMinAge time.Duration) (bool, error) {

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

	if nodeAge < nodeMinAge {
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
			log.Infoln(kuberhealthyEndpoint, "is not ready yet: " + err.Error())
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

	kubeProxyPod, err := client.CoreV1().Pods("kube-system").Get(podName, v1.GetOptions{})
	if err != nil {
		errorMessage := "Failed to get kube-proxy pod: " + podName + ". Error: " + err.Error()
		log.Errorln(errorMessage)
		return kubeProxyReady, errors.New(errorMessage)
	}

	if kubeProxyPod.Status.Phase == corev1.PodRunning && pod.IsPodReady(kubeProxyPod) {
		log.Infoln(kubeProxyPod.Name, "is in status running and ready.")
		kubeProxyReady = true
		return kubeProxyReady, nil
	}

	log.Infoln(kubeProxyPod.Name, "is in status:", kubeProxyPod.Status.Phase, "and ready: ",
		pod.IsPodReady(kubeProxyPod))
	return kubeProxyReady, nil
}
