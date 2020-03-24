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

	// required for oidc kubectl testing
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"k8s.io/client-go/kubernetes"

	checkclient "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
)

const maxTimeInFailure = 60 * time.Second

var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var CheckTimeout time.Duration
var Hostname string

// Checker validates that DNS is functioning correctly
type Checker struct {
	client           *kubernetes.Clientset
	MaxTimeInFailure time.Duration
	Hostname         string
}

func init() {

	// Grab and verify environment variables and set them as global vars
	checkTimeout := os.Getenv("CHECK_POD_TIMEOUT")
	if len(checkTimeout) == 0 {
		log.Errorln("ERROR: The CHECK_TIMEOUT environment variable has not been set.")
		return
	}

	var err error
	CheckTimeout, err = time.ParseDuration(checkTimeout)
	if err != nil {
		log.Errorln("Error parsing timeout for check", checkTimeout, err)
		return
	}

	Hostname = os.Getenv("HOSTNAME")
	if len(Hostname) == 0 {
		log.Errorln("ERROR: The ENDPOINT environment variable has not been set.")
		return
	}
}

func main() {
	client, err := kubeClient.Create(KubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	dc := New()

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
