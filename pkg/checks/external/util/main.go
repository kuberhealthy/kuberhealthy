package util

import (
	"context"
	"io/ioutil"
	"os"
	"os/user"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	podAPIVersion = "v1"
	podKind       = "Pod"
)

// GetOwnerRef fetches the UID from the pod and returns OwnerReference
func GetOwnerRef(client *kubernetes.Clientset, namespace string) ([]metav1.OwnerReference, error) {
	// TODO: refactor function to receive context on exported function in next breaking change.
	ctx := context.TODO()
	podName, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	podSpec, err := getKuberhealthyPod(ctx, client, namespace, strings.ToLower(podName))
	if err != nil {
		return nil, err
	}
	return []metav1.OwnerReference{
		{
			APIVersion: podAPIVersion,
			Kind:       podKind,
			Name:       podSpec.GetName(),
			UID:        podSpec.GetUID(),
		},
	}, nil
}

// getKuberhealthyPod fetches the podSpec
func getKuberhealthyPod(ctx context.Context, client *kubernetes.Clientset, namespace, podName string) (*apiv1.Pod, error) {
	podClient := client.CoreV1().Pods(namespace)
	kHealthyPod, err := podClient.Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return kHealthyPod, nil
}

// GetCurrentUser checks which os use that is running the app
func GetCurrentUser(defaultUser int64) (int64, error) {
	currentUser, err := user.Current()
	if err != nil {
		return 0, err
	}
	intCurrentUser, err := strconv.ParseInt(currentUser.Uid, 0, 64)
	if err != nil {
		return 0, err
	}
	if intCurrentUser == 0 {
		return defaultUser, nil
	}
	return intCurrentUser, nil

}

func GetInstanceNamespace(defaultNamespace string) string {

	instanceNamespace := defaultNamespace

	// instanceNamespaceEnv is a variable for storing namespace instance information
	var instanceNamespaceEnv string

	data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		log.Warnln("Failed to open namespace file:", err.Error())
	}
	if len(data) != 0 {
		instanceNamespaceEnv = string(data)
	}
	if len(instanceNamespaceEnv) != 0 {
		log.Infoln("Found instance namespace:", string(data))
		instanceNamespace = instanceNamespaceEnv
	} else {
		log.Infoln("Did not find instance namespace. Using default namespace:", defaultNamespace)
	}

	return instanceNamespace
}

// PodNameExists determines if a pod with the specified name exists in the specified namespace.
func PodNameExists(client *kubernetes.Clientset, podName string, namespace string) (bool, error) {
	// TODO: refactor function to receive context on exported function in next breaking change.
	ctx := context.TODO()

	// setup a pod watching client for our current KH pod
	podClient := client.CoreV1().Pods(namespace)

	// if the pod is "not found", then it does not exist
	p, err := podClient.Get(ctx, podName, metav1.GetOptions{})
	if err != nil && (k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found")) {
		log.Warnln("Pod", podName, "in namespace", namespace, "was not found"+":", err.Error())
		return false, err
	}

	if err != nil {
		log.Warnln("Error getting pod:", err)
		return false, err
	}

	if p.Name == "" {
		log.Warnln("Pod name is empty. Pod does not exist")
		return false, err
	}

	// if the pod has succeeded, it no longer exists
	if p.Status.Phase == apiv1.PodSucceeded {
		log.Infoln("Pod", podName, "exited successfully.")
		return false, err
	}

	// if the pod has failed, it no longer exists
	if p.Status.Phase == apiv1.PodFailed {
		log.Infoln("Pod", podName, "failed and no longer exists.")
		return false, err
	}
	log.Infoln("Pod", podName, "is present in namespace", namespace)
	return true, nil
}

// PodKill waits a number of seconds determined by the user, then deletes the chosen pod in the namespace specified
func PodKill(client *kubernetes.Clientset, podName string, namespace string, gracePeriod int64) error {
	// TODO: refactor function to receive context on exported function in next breaking change.
	ctx := context.TODO()

	// Setup a pod watching client for our current KH pod
	podClient := client.CoreV1().Pods(namespace)

	// Check for and return any errors, otherwise pod was killed successfully
	err := podClient.Delete(ctx, podName, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod})
	if err != nil && (k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found")) {
		log.Warnln("Pod", podName, "not found not found in namespace", namespace+":", err.Error())
		return err
	}
	log.Infoln("Pod", podName, "was killed successfully.")
	return nil
}
