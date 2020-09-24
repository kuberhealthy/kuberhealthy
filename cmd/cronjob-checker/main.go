package main

import (
	"context"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/events/v1beta1"
	"k8s.io/client-go/rest"
)

// kubeConfigFile is a variable containing file path of Kubernetes config files
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// Namespace is a variable to allow code to target all namespaces or a single namespace
var namespace string

func main() {

	var restInt rest.Interface

	//create events client
	client := v1beta1.New(restInt)


}

func listCronJobs(c *v1beta1.EventsV1beta1Client, namespace string) {
	log.Infoln("Listing CronJobs")

	var opts1 v1.GetOptions

	//get CronJobs
	c := c.Events().Get("cronjob",opts1.Kind("CronJob"))

	var opts2 v1.ListOptions

	//find events from CronJobs
	cronJobs, err := c.Events(namespace).List(opts2)
	if err != nil {
		log.Errorln("Failed to list CronJobs")
	}

	for _, c := range cronJobs.Items {
		if c.
	}
}
