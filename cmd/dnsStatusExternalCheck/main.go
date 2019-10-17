// Package dnsStatus implements a DNS checker for Kuberhealthy
// It verifies that local DNS and external DNS are functioning correctly
package main

import (
	"net"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	// required for oidc kubectl testing
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"k8s.io/client-go/kubernetes"

	checkclient "github.com/Comcast/kuberhealthy/pkg/checks/external/checkClient"
	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
)

const maxTimeInFailure = 60 * time.Second
var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var CheckTimeout time.Duration
var Endpoint string

// Checker validates that DNS is functioning correctly
type Checker struct {
	client           *kubernetes.Clientset
	MaxTimeInFailure time.Duration
	Endpoint       	 string
}

func init() {

	// Grab and verify environment variables and set them as global vars
	checkTimeout := os.Getenv("CHECK_TIMEOUT")
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

	Endpoint = os.Getenv("ENDPOINT")
	if len(Endpoint) == 0 {
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
		log.Errorln("Error running DNS Status check for endpoint:", Endpoint)
	}
	log.Infoln("Done running DNS Status check for endpoint:", Endpoint)
}

// New returns a new DNS Checker
func New() *Checker {
	return &Checker{
		Endpoint:        Endpoint,
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
		return err
	}
}

// doChecks does validations on DNS calls to various endpoints
func (dc *Checker) doChecks() error {

	log.Infoln("DNS Status check testing endpoint:", dc.Endpoint)

	_, err := net.LookupHost(dc.Endpoint)
	if err != nil {
		errorMessage := "DNS Status check determined that " + dc.Endpoint + " is DOWN: " + err.Error()
		log.Errorln(errorMessage)
		err = checkclient.ReportFailure([]string{errorMessage})
		if err != nil {
			log.Println("Error reporting failure to Kuberhealthy servers:", err)
			return err
		}
		log.Println("Successfully reported failure to Kuberhealthy servers")
	}

	log.Infoln("DNS Status check determined that", dc.Endpoint, "was OK.")
	err = checkclient.ReportSuccess()
	if err != nil {
		log.Println("Error reporting success to Kuberhealthy servers:", err)
		return err
	}
	log.Println("Successfully reported success to Kuberhealthy servers")
	return err

}
