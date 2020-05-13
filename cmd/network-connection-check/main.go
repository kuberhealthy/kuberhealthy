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
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	// required for oidc kubectl testing
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"k8s.io/client-go/kubernetes"

	checkclient "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
)

var kubeConfigFile = os.Getenv("KUBECONFIG")
var checkTimeout time.Duration
var connectionTarget string
var targetUnreachable bool

// Checker validates that DNS is functioning correctly
type Checker struct {
	client            *kubernetes.Clientset
	connectionTarget  string
	targetUnreachable bool
}

func init() {

	var err error

	// Grab and verify environment variables and set them as global vars
	CheckTimeout := os.Getenv("CONNECTION_TIMEOUT")
	if len(CheckTimeout) == 0 {
		CheckTimeout = "20s"
		log.Infoln("CONNECTION_TIMEOUT environment variable has not been set. Use", CheckTimeout, "as default timeout.")
	}

	checkTimeout, err = time.ParseDuration(CheckTimeout)
	if err != nil {
		log.Errorln("Error parsing timeout for check", CheckTimeout, err)
		return
	}

	connectionTarget = os.Getenv("CONNECTION_TARGET")
	if len(connectionTarget) == 0 {
		log.Errorln("CONNECTION_TARGET environment variable has not been set.")
		return
	}

	targetUnreachable, err = strconv.ParseBool(os.Getenv("CONNECTION_TARGET_UNREACHABLE"))
	if err != nil {
		log.Infoln("CONNECTION_TARGET_UNREACHABLE could not parsed.")
		return
	}

}

func main() {
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	// create a new network connection checker
	ncc := New()

	err = ncc.Run(client)
	if err != nil {
		log.Errorln("Error running network connection check for:", connectionTarget)
	}
	log.Infoln("Done running network connection check for:", connectionTarget)
}

// New returns a new network connection checker
func New() *Checker {
	return &Checker{
		connectionTarget:  connectionTarget,
		targetUnreachable: targetUnreachable,
	}
}

// Run implements the entrypoint for check execution
func (ncc *Checker) Run(client *kubernetes.Clientset) error {
	log.Infoln("Running network connection checker")

	doneChan := make(chan error)

	ncc.client = client
	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := ncc.doChecks()
		doneChan <- err
	}(doneChan)

	// wait for either a timeout or job completion
	select {
	case <-time.After(checkTimeout + (2000 * time.Millisecond)):

		// The check has timed out after its specified timeout period
		errorMessage := "Failed to complete network connection check in time! Timeout was reached."

		err := checkclient.ReportFailure([]string{errorMessage})
		if err != nil {
			log.Println("Error reporting failure to Kuberhealthy servers:", err)
			return err
		}
		return err
	case err := <-doneChan:
		if err != nil && ncc.targetUnreachable != true {
			return reportKHFailure(err.Error())
		}
		return reportKHSuccess()
	}
}

// doChecks does validations on the network connection call to the endpoint
func (ncc *Checker) doChecks() error {

	network, address := splitAddress(ncc.connectionTarget)

	var localAddr net.Addr
	if network == "udp" {
		localAddr = &net.UDPAddr{IP: net.ParseIP(ncc.connectionTarget)}
	} else {
		localAddr = &net.TCPAddr{IP: net.ParseIP(ncc.connectionTarget)}
	}

	d := net.Dialer{LocalAddr: localAddr, Timeout: time.Duration(checkTimeout)}
	conn, err := d.Dial(network, address)
	if err != nil {
		errorMessage := "Network connection check determined that " + ncc.connectionTarget + " is DOWN: " + err.Error()
		log.Errorln(errorMessage)
		return errors.New(errorMessage)
	}
	err = conn.Close()
	if err != nil {
		return errors.New(err.Error())
	}
	return nil
}

// split network address into transport protocol and network address (with port)
func splitAddress(fulladdress string) (network, address string) {
	split := strings.SplitN(fulladdress, "://", 2)
	if len(split) == 2 {
		return split[0], split[1]
	}
	return "tcp", fulladdress
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
