// Copyright 2020 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package ssl-expiry-check implements an SSL expiration checker for Kuberhealthy
// It verifies that a domain's SSL cert is valid, and does not expire in the next 60 days

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/ssl_util"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
)

const defaultCheckTimeout = 10 * time.Minute

var (
	domainName    string
	portNum       string
	daysToExpire  string
	insecureCheck string
	insecureBool  bool
)

// CheckTimeout is a variable for how long code should run before it should retry.
var CheckTimeout time.Duration

func init() {
	domainName = os.Getenv("DOMAIN_NAME")
	if len(domainName) == 0 {
		log.Error("ERROR: The DOMAIN_NAME environment variable has not been set.")
		return
	}
	portNum = os.Getenv("PORT")
	if len(portNum) == 0 {
		log.Error("ERROR: The PORT environment variable has not been set.")
		return
	}
	daysToExpire = os.Getenv("DAYS")
	if len(daysToExpire) == 0 {
		log.Error("ERROR: The DAYS environment variable has not been set.")
		return
	}
	insecureCheck = os.Getenv("INSECURE")
	if len(insecureCheck) == 0 {
		log.Error("ERROR: The INSECURE environment variable has not been set.")
		return
	}

	insecureBool, _ = strconv.ParseBool(insecureCheck)

	// set debug mode for nodeCheck pkg
	nodeCheck.EnableDebugOutput()

	// Set check time limit to default
	CheckTimeout = defaultCheckTimeout

	// Get the deadline time in unix from the env var
	timeDeadline, err := checkclient.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}

	CheckTimeout = timeDeadline.Sub(time.Now().Add(time.Second * 5))
	log.Infoln("Check time limit set to:", CheckTimeout)
}

func main() {
	// create context
	checkTimeLimit := time.Minute * 1
	ctx, _ := context.WithTimeout(context.Background(), checkTimeLimit)

	// create Kubernetes client
	kubernetesClient, err := kubeClient.Create("")
	if err != nil {
		log.Errorln("Error creating kubeClient with error" + err.Error())
	}

	// hits kuberhealthy endpoint to see if node is ready
	err = nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		log.Errorln("Error waiting for kuberhealthy endpoint to be contactable by checker pod with error:" + err.Error())
	}

	// fetches kube proxy to see if it is ready
	err = nodeCheck.WaitForKubeProxy(ctx, kubernetesClient, "kuberhealthy", "kube-system")
	if err != nil {
		log.Errorln("Error waiting for kube proxy to be ready and running on the node with error:" + err.Error())
	}

	err = runExpiry()
	if err != nil {
		reportErr := reportKHFailure(err.Error())
		if reportErr != nil {
			log.Error(reportErr)
		}
		os.Exit(1)
	}
	reportErr := reportKHSuccess()
	if reportErr != nil {
		log.Error(reportErr)

	}
	os.Exit(0)
}

// runExpiry runs the SSL expiry check from the ssl_util package with the specified env variables
func runExpiry() error {
	certExpired, expirePending, err := ssl_util.CertExpiry(domainName, portNum, daysToExpire, insecureBool)
	if err != nil {
		log.Error("Unable to perform SSL expiration check")
		return err
	}

	if certExpired == true {
		err := fmt.Errorf("Certificate for domain " + domainName + " is expired")
		return err
	}

	if expirePending == true {
		err := fmt.Errorf("Certificate for domain " + domainName + " is expiring in less than " + daysToExpire + " days")
		return err
	}

	return err
}

// reportKHSuccess reports success to Kuberhealthy servers and verifies the report successfully went through
func reportKHSuccess() error {
	err := checkclient.ReportSuccess()
	if err != nil {
		log.Error("Error reporting success status to Kuberhealthy servers:", err)
		return err
	}
	log.Info("Successfully reported success status to Kuberhealthy servers")
	return err
}

// reportKHFailure reports failure to Kuberhealthy servers and verifies the report successfully went through
func reportKHFailure(errorMessage string) error {
	err := checkclient.ReportFailure([]string{errorMessage})
	if err != nil {
		log.Error("Error reporting failure status to Kuberhealthy servers:", err)
		return err
	}
	log.Info("Successfully reported failure status to Kuberhealthy servers")
	return err
}
