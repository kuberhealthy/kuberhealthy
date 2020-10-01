package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// kubeConfigFile is a variable containing file path of Kubernetes config files
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// cronJob name
var cronJob = os.Getenv("CRONJOB")

// namespace is a variable to allow code to target all namespaces or a single namespace
var namespace = os.Getenv("NAMESPACE")

// reason is a varaible to search for event types
var reason = os.Getenv("REASON")

func main() {

	// create kubernetes client
	kubernetesClient, err := kubeClient.Create("")
	if err != nil {
		log.Errorln("Error creating kubeClient with error" + err.Error())
	}

	// //create events client
	client := kubernetesClient.EventsV1beta1()

	e := client.Events(namespace)

	// var getOpts v1.GetOptions
	listOpts := v1.ListOptions{}

	//range over event list
	eventList, err := e.List(context.TODO(), listOpts)
	if err != nil {
		log.Errorln("Error listing events")
	}

	for _, e := range eventList.Items {
		if reason == e.Reason && e.GetName() == cronJob {
			reportErr := fmt.Errorf("CronJob: " + cronJob + "has an event with reason:" + reason)
			ReportFailureAndExit(reportErr)
		}
	}

	err = kh.ReportSuccess()
	if err != nil {
		log.Fatalln("error when reporting to kuberhealthy:", err.Error())
	}
	log.Infoln("Successfully reported to Kuberhealthy")
}

// ReportFailureAndExit logs and reports an error to kuberhealthy and then exits the program.
// If a error occurs when reporting to kuberhealthy, the program fatals.
func ReportFailureAndExit(err error) {
	log.Errorln(err)
	err2 := kh.ReportFailure([]string{err.Error()})
	if err2 != nil {
		log.Fatalln("error when reporting to kuberhealthy:", err.Error())
	}
	os.Exit(0)
}
