// Package ssl-expiry-check implements an SSL expiration checker for Kuberhealthy
// It verifies that a domain's SSL cert is valid, and does not expire in the next 60 days

package main

import (
	"context"
	"fmt"
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
	domainName     string
	portNum        string
	daysToExpire   string
	insecureCheck  string
	insecureBool   bool
	certExpired    bool
	expireWarning  bool
	checkNamespace string
	ctx            context.Context
)

type Checker struct {
	client        *kubernetes.Clientset
	domainName    string
	portNum       string
	certExpired   bool
	expireWarning bool
}

func init() {
	// Set check time limit to default
	checkTimeout = time.Duration(time.Second * 20)

	// Get the deadline time in unix from the env var
	timeDeadline, err := checkclient.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}
	checkTimeout = timeDeadline.Sub(time.Now().Add(time.Second * 5))
	log.Infoln("Check time limit set to:", checkTimeout)

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

	sec := New()
	var cancelFunc context.CancelFunc
	nodeCheckCtx, cancelFunc = context.WithTimeout(context.Background(), checkTimeout)

	err = sec.runExpiry(nodeCheckCtx, cancelFunc, client)
	if err != nil {
		log.Errorln("Error completing SSL expiry check for", domainName+":", err)
	}

}

func New() *Checker {
	return &Checker{
		domainName:    domainName,
		portNum:       portNum,
		certExpired:   certExpired,
		expireWarning: expireWarning,
	}
}

// runExpiry runs the SSL expiry check from the ssl_util package with the specified env variables
func (sec *Checker) runExpiry(ctx context.Context, cancel context.CancelFunc, client *kubernetes.Clientset) error {
	doneChan := make(chan error)
	runTimeout := time.After(checkTimeout)

	go func(doneChan chan error) {
		err := sec.doChecks()
		doneChan <- err
	}(doneChan)

	select {
	case <-ctx.Done():
		log.Println("Cancelling check and shutting down due to interrupt.")
		return reportKHFailure("Cancelling check and shutting down due to interrupt.")
	case <-runTimeout:
		cancel()
		log.Println("Cancelling check and shutting down due to timeout.")
		return reportKHFailure("Failed to complete SSL expiry check in time. Timeout was reached.")
	case err := <-doneChan:
		cancel()
		if err != nil {
			return reportKHFailure(err.Error())
		}
		return reportKHSuccess()
	}
}

func (sec *Checker) doChecks() error {
	certExpired, expirePending, err := ssl_util.CertExpiry(domainName, portNum, daysToExpire, insecureBool)
	if err != nil {
		log.Error("Unable to perform SSL expiration check")
		return err
	}

	if certExpired {
		err := fmt.Errorf("Certificate for domain " + domainName + " is expired")
		return err
	}

	if expirePending {
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

func waitForNodeToJoin(ctx context.Context) {

	// Check if Kuberhealthy is reachable.
	err := nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		log.Errorln("Failed to reach Kuberhealthy:", err.Error())
	}
}
