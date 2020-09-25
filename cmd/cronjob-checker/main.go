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

	// //create events client
	client := v1beta1.New(restInt)

	e := client.Events(namespace)

	var getOpts v1.GetOptions

	events, err := e.Get(context.TODO(), "check-reaper", getOpts)
	if err != nil {
		log.Errorln("Error geting events")
	}

}

func listCronJobs(c *v1beta1.EventsV1beta1Client, namespace string, cronJobName string) {
	log.Infoln("Listing CronJobs")

	var listOpts v1.ListOptions

	//get CronJobs
	i, err := c.Events(namespace).List(context.TODO(), listOpts)
	if err != nil {
		log.Errorln("Error listing CronJob Events")
	}
}
