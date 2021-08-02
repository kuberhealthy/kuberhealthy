// Package dnsStatus implements a DNS checker for Kuberhealthy
// It verifies that local DNS and external DNS are functioning correctly
package main

import (
	"context"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	// required for oidc kubectl testing
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"k8s.io/client-go/kubernetes"

	checkclient "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
)

var (
	kubeConfigFile    = os.Getenv("KUBECONFIG")
	checkTimeout      time.Duration
	connectionTarget  string
	targetUnreachable bool
	checkNamespace    string
	ctx               context.Context
)

const (
	errorMessage = "Failed to complete network connection check in time! Timeout was reached."
)

// Checker validates that DNS is functioning correctly
type Checker struct {
	client            *kubernetes.Clientset
	connectionTarget  string
	targetUnreachable bool
}

func init() {

	var err error

	// Set check time limit to default
	checkTimeout = time.Duration(time.Second * 20)
	// Get the deadline time in unix from the env var
	timeDeadline, err := checkclient.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}
	checkTimeout = timeDeadline.Sub(time.Now().Add(time.Second * 5))
	log.Infoln("Check time limit set to:", checkTimeout)

	connectionTarget = os.Getenv("CONNECTION_TARGET")
	if len(connectionTarget) == 0 {
		log.Errorln("CONNECTION_TARGET environment variable has not been set.")
		return
	}

	targetUnreachable, err = strconv.ParseBool(os.Getenv("CONNECTION_TARGET_UNREACHABLE"))
	if err != nil {
		log.Infoln("CONNECTION_TARGET_UNREACHABLE could not be parsed.")
		return
	}

	data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		log.Warnln("Failed to open namespace file:", err.Error())
	}
	if len(data) != 0 {
		log.Infoln("Found pod namespace:", string(data))
		checkNamespace = string(data)
	}

	nodeCheck.EnableDebugOutput()
}

func main() {
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	// create a new network connection checker
	ncc := New()
	var cancelFunc context.CancelFunc
	ctx, cancelFunc = context.WithTimeout(context.Background(), checkTimeout)

	// wait for the node to join the worker pool
	waitForNodeToJoin(ctx)
	err = ncc.Run(ctx, cancelFunc, client)
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
func (ncc *Checker) Run(ctx context.Context, cancel context.CancelFunc, client *kubernetes.Clientset) error {
	log.Infoln("Running network connection checker")

	doneChan := make(chan error)
	runTimeout := time.After(checkTimeout)

	ncc.client = client
	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := ncc.doChecks()
		doneChan <- err
	}(doneChan)

	// wait for either a timeout or job completion
	select {
	case <-ctx.Done():
		log.Println("Cancelling check and shutting down due to interrupt.")
		return reportKHFailure("Cancelling check and shutting down due to interrupt.")
	case <-runTimeout:
		cancel()
		log.Println("Cancelling check and shutting down due to timeout.")
		return reportKHFailure("Failed to complete network connection check in time! Timeout was reached.")
	case err := <-doneChan:
		cancel()
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

// waitForNodeToJoin waits for the node to join the worker pool.
// Waits for kube-proxy to be ready and that Kuberhealthy is reachable.
func waitForNodeToJoin(ctx context.Context) {

	// Check if Kuberhealthy is reachable.
	err := nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		log.Errorln("Failed to reach Kuberhealthy:", err.Error())
	}
}
