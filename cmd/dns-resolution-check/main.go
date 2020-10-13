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
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	// required for oidc kubectl testing
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"k8s.io/client-go/kubernetes"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
)

const maxTimeInFailure = 60 * time.Second
const defaultCheckTimeout = 5 * time.Minute

// KubeConfigFile is a variable containing file path of Kubernetes config files
var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// CheckTimeout is a variable for how long code should run before it should retry
var CheckTimeout time.Duration

// Hostname is a variable for container/pod name
var Hostname string

// NodeName is a variable for the node where the container/pod is created
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

	Hostname = os.Getenv("HOSTNAME")
	if len(Hostname) == 0 {
		log.Errorln("ERROR: The ENDPOINT environment variable has not been set.")
		return
	}

	NodeName = os.Getenv("NODE_NAME")
	if len(NodeName) == 0 {
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

	// Since this check runs often and very quickly, run all nodeChecks to make sure that:
	// - check doesn't run on a node that's too young
	// - kuberhealthy endpoint is ready
	// - kube-proxy is ready and running on the node the check is running on
	nodeCheck.EnableDebugOutput()
	checkTimeLimit := time.Minute * 1
	ctx, _ := context.WithTimeout(context.Background(), checkTimeLimit)

	minNodeAge := time.Minute * 3
	err = nodeCheck.WaitForNodeAge(ctx, client, "kuberhealthy", minNodeAge)
	if err != nil {
		log.Errorln("Error waiting for node to reach minimum age:", err)
	}

	err = nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		log.Errorln("Error waiting for Kuberhealthy to be ready:", err)
	}

	err = nodeCheck.WaitForKubeProxy(ctx, client, "kuberhealthy", "kube-system")
	if err != nil {
		log.Errorln("Error waiting for kube proxy to be ready:", err)
	}

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
