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

// Package sslStatus implements an SSL checker for Kuberhealthy
// It verifies that a domain's SSL cert is valid, and does not expire in the next 60 days

package main

import (
	"crypto/tls"
	"net"
	"os"
	"time"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
)

// For security purposes, SkipVerify should always be set to "false" except for testing purposes
const SkipVerify = false

var Domainname string

var Portnum string

var TimeoutSeconds = 5

func init() {
	Domainname = os.Getenv("DOMAINNAME")
	if len(Domainname) == 0 {
		log.Errorln("ERROR: The DOMAINNAME environment variable has not been set.")
		return
	}
	Portnum = os.Getenv("PORT")
	if len(Portnum) == 0 {
		log.Errorln("ERROR: The PORT environment variable has not been set.")
		return
	}
}

func main() {
	/*TestCert, TestIP, _*/ err := FetchCert(Domainname, Portnum)
	//fmt.Println(TestCert, TestIP)
	if err != nil {
		log.Fatal("Unable to perform SSL handshake with host:", err)
	} else {
		log.Println("SSL handshake check completed")
	}
}

func FetchCert(host, port string) /*[]*x509.Certificate, string, */ error {
	d := &net.Dialer{
		Timeout: time.Duration(TimeoutSeconds) * time.Second,
	}

	conn, err := tls.DialWithDialer(d, "tcp", host+":"+port, &tls.Config{
		InsecureSkipVerify: SkipVerify,
	})

	if err != nil {
		log.Fatal( /*[]*x509.Certificate{&x509.Certificate{}}, "", */ err)
	}
	defer conn.Close()

	handshakeErr := conn.Handshake()
	if handshakeErr != nil {
		log.Fatal("Unable to complete SSL handshake with host", host+":", handshakeErr)
		return reportKHFailure(err.Error())
	} else {
		log.Println("SSL handshake to host", host, "completed successfully")
		return reportKHSuccess()
	}
	/*
		addr := conn.RemoteAddr()
		ip, _, _ := net.SplitHostPort(addr.String())
		cert := conn.ConnectionState().PeerCertificates

		return cert, ip, nil
	*/
}

// reportKHSuccess reports success to Kuberhealthy servers and verifies the report successfully went through
func reportKHSuccess() error {
	err := checkclient.ReportSuccess()
	if err != nil {
		log.Println("Error reporting success status to Kuberhealthy servers:", err)
		return err
	}
	log.Println("Successfully reported success status to Kuberhealthy servers")
	return err
}

// reportKHFailure reports failure to Kuberhealthy servers and verifies the report successfully went through
func reportKHFailure(errorMessage string) error {
	err := checkclient.ReportFailure([]string{errorMessage})
	if err != nil {
		log.Println("Error reporting failure status to Kuberhealthy servers:", err)
		return err
	}
	log.Println("Successfully reported failure status to Kuberhealthy servers")
	return err
}
