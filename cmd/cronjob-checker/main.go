package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/events"
)

// kubeConfigFile is a variable containing file path of Kubernetes config files
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

func main() {
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	events

}

func listCronJobs(client *kubernetes.Clientset, namespace string) (map[string]v1.Pod, error) {

	log.Infoln("Listing CronJobs")

	CronJobs := make(map[string]v1.Pod)
	cj, err := client.BatchV1beta1().CronJobs(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Errorln("Failed to list CronJobs")
		return CronJobs, err
	}
	log.Infoln("Found:", len(cj.Items), "cronjobs in namespace:", namespace)

	cj.Descriptor()

	for _, c := range cj.Items {
		if c.GetLabels()
	}
}
