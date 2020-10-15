package nodeCheck

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external"
)

// EnableDebugOutput enables debug logging for this package
func EnableDebugOutput() {
	log.SetLevel(log.DebugLevel)
}

// WaitForKuberhealthy waits for the the kuberhealthy endpoint (KH_REPORTING_URL) to be contactable by the checker pod
// on a given node
func WaitForKuberhealthy(ctx context.Context) error {

	kuberhealthyEndpoint := os.Getenv(external.KHReportingURL)

	// check the length of the reporting url to make sure we pulled one properly
	if len(kuberhealthyEndpoint) < 1 {
		return errors.New("error getting kuberhealthy reporting URL from environment variable " +
			external.KHReportingURL + " was blank")
	}

	log.Debugln("Checking if the kuberhealthy endpoint:", kuberhealthyEndpoint, "is ready.")
	select {
	case err := <-waitForKuberhealthyEndpointReady(ctx, kuberhealthyEndpoint):
		if err != nil {
			return err
		}
		log.Debugln("Kuberhealthy endpoint:", kuberhealthyEndpoint, "is ready. Proceeding to run check.")
	case <-ctx.Done():
		return errors.New("context cancelled waiting for Kuberhealthy endpoint to be ready")
	}
	return nil
}

// WaitForKubeProxy waits for kube proxy to be ready and running on the node before running the check. Assumes that the
// kube-proxy pod follows the naming convention: "kube-proxy-{nodeName}"
func WaitForKubeProxy(ctx context.Context, client *kubernetes.Clientset, KHNamespace string, kubeProxyNamespace string) error {

	khPod, err := getKHPod(client, KHNamespace)
	if err != nil {
		return errors.New("error getting kuberhealthy pod: " + err.Error())
	}
	log.Debugln("Checking if kube-proxy is running and ready on this node:", khPod.Spec.NodeName)
	select {
	case err := <-waitForKubeProxyReady(ctx, client, khPod.Spec.NodeName, kubeProxyNamespace):
		if err != nil {
			return err
		}
		log.Debugln("Kube proxy is ready. Proceeding to run check.")
	case <-ctx.Done():
		return errors.New("context cancelled waiting for kube proxy to be ready and running")
	}
	return nil
}

// WaitForNodeAge checks the node's age to see if its less than the minimum node age. If so, sleeps until the node
// reaches the minimum node age.
func WaitForNodeAge(ctx context.Context, client *kubernetes.Clientset, namespace string, minNodeAge time.Duration) error {

	khPod, err := getKHPod(client, namespace)
	if err != nil {
		return err
	}
	log.Debugln("Pod is on node:", khPod.Spec.NodeName)

	node, err := client.CoreV1().Nodes().Get(ctx, khPod.Spec.NodeName, v1.GetOptions{})
	if err != nil {
		return err
	}
	// get current age of the node
	nodeAge := time.Now().Sub(node.CreationTimestamp.Time)
	log.Debugln("Check running on node: ", node.Name, "with node age:", nodeAge)
	if nodeAge >= minNodeAge {
		return nil
	}

	select {
	case <-ctx.Done():
		return errors.New("context cancelled waiting for node to reach minNodeAge")
	default:
		sleepDuration := minNodeAge - nodeAge
		log.Debugln("Node is new. Sleeping for:", sleepDuration, "until node reaches minNodeAge:", minNodeAge)
		time.Sleep(sleepDuration)
	}
	return nil
}

// getKHPod gets the kuberhealthy pod currently running. The pod is needed to get the pod's node information.
func getKHPod(client *kubernetes.Clientset, namespace string) (*corev1.Pod, error) {

	var khPod *corev1.Pod
	podName, err := os.Hostname()
	if err != nil {
		return khPod, err
	}

	log.Debugln("Getting pod:", podName, "in order to get its node information")
	khPod, err = client.CoreV1().Pods(namespace).Get(context.TODO(), podName, v1.GetOptions{})
	if err != nil {
		return khPod, err
	}
	return khPod, err
}

// waitForKuberhealthyEndpointReady hits the kuberhealthy endpoint every 3 seconds to see if the node is ready to reach
// the endpoint.
func waitForKuberhealthyEndpointReady(ctx context.Context, kuberhealthyEndpoint string) chan error {

	doneChan := make(chan error, 1)

	for {
		select {
		case <-ctx.Done():
			doneChan <- errors.New("context cancelled waiting for Kuberhealthy endpoint to be ready")
			return doneChan
		default:
		}

		_, err := http.NewRequest("GET", kuberhealthyEndpoint, nil)
		if err == nil {
			log.Debugln(kuberhealthyEndpoint, "is ready.")
			doneChan <- nil
			return doneChan
		} else {
			log.Debugln(kuberhealthyEndpoint, "is not ready yet..."+err.Error())
		}
		time.Sleep(time.Second * 3)
	}
}

// waitForKubeProxyReady fetches the kube proxy pod every 5 seconds until it's ready and running.
func waitForKubeProxyReady(ctx context.Context, client *kubernetes.Clientset, nodeName string, kubeProxyNamespace string) chan error {

	kubeProxyName := "kube-proxy-" + nodeName
	doneChan := make(chan error, 1)

	for {
		select {
		case <-ctx.Done():
			doneChan <- errors.New("context cancelled waiting for kube proxy to be ready and running")
			return doneChan
		default:
		}
		kubeProxyReady, err := checkKubeProxyPod(client, kubeProxyName, kubeProxyNamespace)
		if err != nil {
			doneChan <- err
			return doneChan
		}

		if kubeProxyReady {
			log.Debugln("Kube proxy: ", kubeProxyName, "is running and ready!")
			doneChan <- nil
			return doneChan
		}
		time.Sleep(time.Second * 5)
	}
}

// checkKubeProxyPod gets the kube proxy pod and checks if its ready and running.
func checkKubeProxyPod(client *kubernetes.Clientset, podName string, kubeProxyNamespace string) (bool, error) {

	var kubeProxyReady bool

	kubeProxyPod, err := client.CoreV1().Pods(kubeProxyNamespace).Get(context.TODO(), podName, v1.GetOptions{})
	if err != nil {
		return kubeProxyReady, errors.New("Failed to get kube-proxy pod: " + podName + ". Error: " + err.Error())
	}

	var podReady = true
	for _, cs := range kubeProxyPod.Status.Conditions {
		if cs.Status != "True" {
			podReady = false
			break
		}
	}

	if kubeProxyPod.Status.Phase == corev1.PodRunning && podReady {
		kubeProxyReady = true
		return kubeProxyReady, nil
	}

	log.Debugln(kubeProxyPod.Name, "is in status:", kubeProxyPod.Status.Phase, "and ready: ", podReady)
	return kubeProxyReady, nil
}
