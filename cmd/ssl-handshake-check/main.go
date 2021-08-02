// Package ssl-handshake-check implements an SSL TLS handshake checker for Kuberhealthy
// It verifies that a domain's SSL cert is valid, and does not expire in the next 60 days

package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/ssl_util"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

var (
	kubeConfigFile = os.Getenv("KUBECONFIG")
	checkTimeout   time.Duration
	checkNamespace string
	domainName     string
	portNum        string
	selfSigned     string
	selfSignedBool bool
	ctx            context.Context
)

type Checker struct {
	client         *kubernetes.Clientset
	domainName     string
	portNum        string
	selfSignedBool bool
}

func init() {
	// Set the check time limit to default
	checkTimeout = time.Duration(time.Second * 20)

	// Get the deadline time in Unix from the environment variable
	timeDeadline, err := checkclient.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}
	checkTimeout = timeDeadline.Sub(time.Now().Add(time.Second * 5))
	log.Infoln("Check time limit set to: ", checkTimeout)

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
	selfSigned = os.Getenv("SELF_SIGNED")
	if len(selfSigned) == 0 {
		log.Error("ERROR: The SELF_SIGNED environment variable has not been set.")
		return
	}
	selfSignedBool, _ = strconv.ParseBool(selfSigned)

	// set debug mode for nodeCheck pkg
	nodeCheck.EnableDebugOutput()
}

func main() {
	// create context
	nodeCheckTimeout := time.Minute * 1
	nodeCheckCtx, _ := context.WithTimeout(context.Background(), nodeCheckTimeout.Round(10))

	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	// wait for the node to join the worker pool
	waitForNodeToJoin(nodeCheckCtx)

	shc := New()
	var cancelFunc context.CancelFunc
	nodeCheckCtx, cancelFunc = context.WithTimeout(context.Background(), checkTimeout)

	err = shc.runHandshake(nodeCheckCtx, cancelFunc, client)
	if err != nil {
		log.Errorln("Error completing SSL handshake check for", domainName+":", err)
	}
}

func New() *Checker {
	return &Checker{
		domainName:     domainName,
		portNum:        portNum,
		selfSignedBool: selfSignedBool,
	}
}

// runHandshake runs the SSL handshake check for the specified host and port number from ssl_util package
func (shc *Checker) runHandshake(ctx context.Context, cancel context.CancelFunc, client *kubernetes.Clientset) error {
	doneChan := make(chan error)
	runTimeout := time.After(checkTimeout)

	go func(doneChan chan error) {
		err := shc.doChecks()
		doneChan <- err
	}(doneChan)

	select {
	case <-ctx.Done():
		log.Println("Cancelling check and shutting down due to interrupt.")
		return reportKHFailure("Cancelling check and shutting down due to interrupt.")
	case <-runTimeout:
		cancel()
		log.Println("Cancelling check and shutting down due to timeout.")
		return reportKHFailure("Failed to complete SSL handshake in time. Timeout was reached.")
	case err := <-doneChan:
		cancel()
		if err != nil {
			log.Errorln("Error when doing SSL handshake:", err)
			return reportKHFailure(err.Error())
		}
		return reportKHSuccess()
	}
}

func (shc *Checker) doChecks() error {
	siteURL, err := url.Parse("https://" + domainName + ":" + portNum)
	if err != nil {
		return err
	}

	// create a cert pool for this check
	certPool, err := ssl_util.CreatePool()
	if err != nil {
		return fmt.Errorf("error creating cert pool for ssl checks: %w", err)
	}

	return ssl_util.SSLHandshakeWithCertPool(siteURL, certPool)
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
	return nil
}

func waitForNodeToJoin(ctx context.Context) {

	// Check if Kuberhealthy is reachable.
	err := nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		log.Errorln("Failed to reach Kuberhealthy:", err.Error())
	}
}
