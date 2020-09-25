package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/events/v1beta1"
	"k8s.io/client-go/rest"
)

// kubeConfigFile is a variable containing file path of Kubernetes config files
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// cronJob name
var cronJob = os.Getenv("CRONJOB")

// Namespace is a variable to allow code to target all namespaces or a single namespace
var namespace = os.Getenv("NAMESPACE")

func main() {

	var restInt rest.Interface

	// //create events client
	client := v1beta1.New(restInt)

	e := client.Events(namespace)

	var getOpts v1.GetOptions

	events, err := e.Get(context.TODO(), cronJob, getOpts)
	if err != nil {
		log.Errorln("Error geting events")
	}

	if events.Reason == "FailedNeedsStart" {
		reportErr := fmt.Errorf("CronJob: " + cronJob + "has an event with reason FailedNeedsStart")
		ReportFailureAndExit(reportErr)
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
