package main

import (
	"context"
	"os"
	"time"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/ssl_util"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
)

var TimeoutSeconds = 10
var domainName string
var portNum string

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
	fileFound := whichCheck()
	err = runCheck(fileFound)
	if err != nil {
		reportErr := reportKHFailure(err.Error())
		if reportErr != nil {
			log.Error(reportErr)
		}
		os.Exit(1)
	}

	if err == nil {
		reportErr := reportKHSuccess()
		if reportErr != nil {
			log.Error(reportErr)
		}
		os.Exit(1)
	}
}

// whichCheck determines if a cert has been provided, or if it needs to be retrieved from the host and returns a bool
func whichCheck() bool {
	var fromFile bool
	if _, err := os.Stat("/etc/ssl/selfsign/certificate.crt"); err == nil {
		fromFile = true
	}
	return fromFile
}

// runCheck takes in a bool from the whichCheck func and calls the one of two handshake check functions
func runCheck(fileFound bool) error {
	var err error

	// if the user provided pem file is present, import it and run the handshake check
	if fileFound {
		err := ssl_util.HandshakeFromFile(domainName, portNum)
		return err
	}

	// if the pem file has not been provided, pull the cert from the host, import it, and for the handshake check
	if !fileFound {
		selfCert, err := ssl_util.CertificatePuller(domainName, portNum)
		if err != nil {
			log.Warn("Error pulling certificate from host: ", err)
		}

		err = ssl_util.HandshakeFromHost(domainName, portNum, selfCert)

		if err != nil {
			log.Warn("Error performing handshake: ", err)
		}
		return err
	}
	return err
}

// reportKHSuccess reports success to Kuberhealthy servers and verifies the report successfully went through
func reportKHSuccess() error {
	err := checkclient.ReportSuccess()
	if err != nil {
		log.Error("Error reporting success status to Kuberhealthy servers: ", err)
		return err
	}
	log.Info("Successfully reported success status to Kuberhealthy servers")
	return err
}

// reportKHFailure reports failure to Kuberhealthy servers and verifies the report successfully went through
func reportKHFailure(errorMessage string) error {
	err := checkclient.ReportFailure([]string{errorMessage})
	if err != nil {
		log.Error("Error reporting failure status to Kuberhealthy servers: ", err)
		return err
	}
	log.Info("Successfully reported failure status to Kuberhealthy servers")
	return err
}
