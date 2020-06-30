package util

import (
	"io/ioutil"
	"os"
	"os/user"
	"strconv"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	log "github.com/sirupsen/logrus"
)

const (
	podAPIVersion = "v1"
	podKind       = "Pod"
)

// GetOwnerRef fetches the UID from the pod and returns OwnerReference
func GetOwnerRef(client *kubernetes.Clientset, namespace string) ([]metav1.OwnerReference, error) {
	podName, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	podSpec, err := getKuberhealthyPod(client, namespace, strings.ToLower(podName))
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
func getKuberhealthyPod(client *kubernetes.Clientset, namespace, podName string) (*apiv1.Pod, error) {
	podClient := client.CoreV1().Pods(namespace)
	kHealthyPod, err := podClient.Get(podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return kHealthyPod, nil
}

// GetCurrentUser checks which os use that is running the app
func GetCurrentUser(defaultUser int64) (int64, error) {
	runAsUser := defaultUser

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
	runAsUser = intCurrentUser
	return runAsUser, nil

}

func GetInstanceNamespace(defaultNamespace string) string {

	instanceNamespace := defaultNamespace
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