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
	"os"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/ssl_util"
	log "github.com/sirupsen/logrus"
)

var domainName string
var portNum string
var daysToExpire string

func init() {
	domainName = os.Getenv("DOMAIN_NAME")
	if len(domainName) == 0 {
		log.Errorln("ERROR: The DOMAIN_NAME environment variable has not been set.")
		return
	}
	portNum = os.Getenv("PORT")
	if len(portNum) == 0 {
		log.Errorln("ERROR: The PORT environment variable has not been set.")
		return
	}
	daysToExpire = os.Getenv("DAYS")
	if len(daysToExpire) == 0 {
		log.Errorln("ERROR: The DAYS environment variable has not been set.")
		return
	}
}

func main() {
	runExpiry()
}

func runExpiry() {
	certExpired, expirePending, err := ssl_util.CertExpiry(domainName, portNum, daysToExpire)
	if err != nil {
		log.Error("Unable to perform SSL expiration check with host")
		os.Exit(1)
	}

	if certExpired == true {
		err := reportKHFailure("Certificate is expired")
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		return
	}

	if expirePending == true {
		errMsg := ("Certificate for domain " + domainName + " is expiring in less than " + daysToExpire + " days")
		err := reportKHFailure(errMsg)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		return
	}
	err = reportKHSuccess()
	if err != nil {
		log.Error(err)
	}
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
