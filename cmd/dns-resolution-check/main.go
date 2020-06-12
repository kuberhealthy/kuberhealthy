// Copyright 2018 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package dnsStatus implements a DNS checker for Kuberhealthy
// It verifies that local DNS and external DNS are functioning correctly
package main

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// required for oidc kubectl testing
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"k8s.io/client-go/kubernetes"

	checkclient "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
)

const maxTimeInFailure = 60 * time.Second
const defaultCheckTimeout = 5 * time.Minute

var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var CheckTimeout time.Duration
var Hostname string
var NodeName string
var now time.Time

// Checker validates that DNS is functioning correctly
type Checker struct {
	client           *kubernetes.Clientset
	MaxTimeInFailure time.Duration
	Hostname         string
}

func init() {

	// Set check time limit to default
	CheckTimeout = defaultCheckTimeout
	// Get the deadline time in unix from the env var
	timeDeadline, err := checkclient.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}
	CheckTimeout = timeDeadline.Sub(time.Now().Add(time.Second * 5))
	log.Infoln("Check time limit set to:", CheckTimeout)
<<<<<<< HEAD
=======

>>>>>>> 753b184b6eb6a6751e10aee8bce147ec451cf02c

	Hostname = os.Getenv("HOSTNAME")
	if len(Hostname) == 0 {
		log.Errorln("ERROR: The ENDPOINT environment variable has not been set.")
		return
	}

	NodeName = os.Getenv("NODE_NAME")
	if len(Hostname) == 0 {
		log.Errorln("ERROR: Failed to retrieve NODE_NAME environment variable.")
		return
	}
	log.Infoln("Check pod is running on node:", NodeName)

	now = time.Now()
}

func main() {
	client, err := kubeClient.Create(KubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	dc := New()

	// Check node age before running check. DNS resolution check runs often, and very quickly. If the node is new, we
	// want to sleep for a minute to give the node extra time to be fully ready for the check run.
	checkNodeAge(client)

	err = dc.Run(client)
	if err != nil {
		log.Errorln("Error running DNS Status check for hostname:", Hostname)
	}
	log.Infoln("Done running DNS Status check for hostname:", Hostname)
}

// New returns a new DNS Checker
func New() *Checker {
	return &Checker{
		Hostname:         Hostname,
		MaxTimeInFailure: maxTimeInFailure,
	}
}

// Run implements the entrypoint for check execution
func (dc *Checker) Run(client *kubernetes.Clientset) error {
	log.Infoln("Running DNS status checker")
	doneChan := make(chan error)

	dc.client = client
	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := dc.doChecks()
		doneChan <- err
	}(doneChan)

	// wait for either a timeout or job completion
	select {
	case <-time.After(CheckTimeout):
		// The check has timed out after its specified timeout period
		errorMessage := "Failed to complete DNS Status check in time! Timeout was reached."
		err := checkclient.ReportFailure([]string{errorMessage})
		if err != nil {
			log.Println("Error reporting failure to Kuberhealthy servers:", err)
			return err
		}
		return err
	case err := <-doneChan:
		if err != nil {
			return reportKHFailure(err.Error())
		}
		return reportKHSuccess()
	}
}

// doChecks does validations on the DNS call to the endpoint
func (dc *Checker) doChecks() error {

	log.Infoln("DNS Status check testing hostname:", dc.Hostname)
	_, err := net.LookupHost(dc.Hostname)
	if err != nil {
		errorMessage := "DNS Status check determined that " + dc.Hostname + " is DOWN: " + err.Error()
		log.Errorln(errorMessage)
		return errors.New(errorMessage)
	}
	log.Infoln("DNS Status check determined that", dc.Hostname, "was OK.")
	return nil
}

// checkNodeAge checks the node's age to make sure its not less than three minutes old. If so, sleep for one minute
// before running check
func checkNodeAge(client *kubernetes.Clientset) {

	node, err := client.CoreV1().Nodes().Get(NodeName, v1.GetOptions{})
	if err != nil {
		log.Errorln("Failed to get node:", NodeName, err)
		return
	}

	nodeMinAge := time.Minute*3
	sleepDuration := time.Minute
	nodeAge := now.Sub(node.CreationTimestamp.Time)
	// if the node the pod is on is less than 3 minutes old, wait 1 minute before running check.
	log.Infoln("Check running on node: ", node.Name, "with node age:", nodeAge)
	if nodeAge < nodeMinAge {
		log.Infoln("Node is than", nodeMinAge, "old. Sleeping for", sleepDuration, "before running check")
		time.Sleep(sleepDuration)
		return
	}
	return
}

// reportKHSuccess reports success to Kuberhealthy servers and verifies the report successfully went through
func reportKHSuccess() error {
	err := checkclient.ReportSuccess()
	if err != nil {
		log.Println("Error reporting success to Kuberhealthy servers:", err)
		return err
	}
	log.Println("Successfully reported success to Kuberhealthy servers")
	return err
}

// reportKHFailure reports failure to Kuberhealthy servers and verifies the report successfully went through
func reportKHFailure(errorMessage string) error {
	err := checkclient.ReportFailure([]string{errorMessage})
	if err != nil {
		log.Println("Error reporting failure to Kuberhealthy servers:", err)
		return err
	}
	log.Println("Successfully reported failure to Kuberhealthy servers")
	return err
}
