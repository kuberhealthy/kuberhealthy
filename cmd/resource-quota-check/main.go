package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	kh "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

const (
	// Default memory and CPU usage alert threshold is set to 90% (inclusive).
	defaultThreshold = 0.9

	// Set the default check time limit to 5 minutes.
	defaultCheckTimeLimit = time.Minute * 5
)

var (
	// K8s config file for the client.
	kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

	// Given namespaces to ignore or to look at. (Determined by BLACKLIST_NAMESPACES or WHITELIST_NAMESPACES environment variables.)
	// Blacklist is enabled by default (looks at all namespaces, except those specified in the blacklist).
	// Expects a comma separated list of values (i.e. "default,kube-system")
	// Ignores namespaces from the blacklist if turned on. (On by default, checks all namespaces)
	blacklistEnv = os.Getenv("BLACKLIST")
	blacklist    []string

	// Looks at specified namespaces from the whitelist if turned on.
	whitelistEnv = os.Getenv("WHITELIST")
	whitelist    []string

	// Threshold for resource quota usage. (inclusive)
	// If given 0.9 (or 90%), this check will alert when usage for memory or CPU is at least 90%.
	thresholdEnv = os.Getenv("THRESHOLD")
	threshold    float64

	// Check time limit.
	checkTimeLimitEnv = os.Getenv("CHECK_TIME_LIMIT")
	checkTimeLimit    time.Duration

	ctx       context.Context
	ctxCancel context.CancelFunc

	// Interrupt signal channels.
	signalChan chan os.Signal
	doneChan   chan bool

	debugEnv = os.Getenv("DEBUG")
	debug    bool

	// K8s client used for the check.
	client *kubernetes.Clientset

	// checkErrors = make([]string, 0)
)

func init() {
	// Parse incoming debug settings.
	parseDebugSettings()

	// Parse all incoming input environment variables and crash if an error occurs
	// during parsing process.
	parseInputValues()

	// Allocate channels.
	signalChan = make(chan os.Signal, 3)
	doneChan = make(chan bool)
}

func main() {

	ctx, ctxCancel = context.WithTimeout(context.Background(), time.Duration(time.Minute*5))

	// Create a kubernetes client.
	var err error
	client, err = kubeClient.Create(kubeConfigFile)
	if err != nil {
		errorMessage := "failed to create a kubernetes client with error: " + err.Error()
		reportErr := kh.ReportFailure([]string{errorMessage})
		if reportErr != nil {
			log.Fatalln("error reporting failure to kuberhealthy:", reportErr.Error())
		}
		return
	}
	log.Infoln("Kubernetes client created.")

	// Catch panics.
	var r interface{}
	defer func() {
		r = recover()
		if r != nil {
			log.Infoln("Recovered panic:", r)
			reportErr := kh.ReportFailure([]string{r.(string)})
			if reportErr != nil {
				log.Fatalln("error reporting failure to kuberhealthy:", reportErr.Error())
			}
		}
	}()

	runResourceQuotaCheck(ctx)
}

func listenForInterrupts(ctx context.Context) {

	// Relay incoming OS interrupt signals to the signalChan.
	signal.Notify(signalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)
	sig := <-signalChan // This is a blocking operation -- the routine will stop here until there is something sent down the channel.
	log.Infoln("Received an interrupt signal from the signal channel.")
	log.Debugln("Signal received was:", sig.String())

	log.Debugln("Cancelling context.")
	ctxCancel() // Causes all functions within the check to return without error and abort. NOT an error
	// condition; this is a response to an external shutdown signal.

	// Clean up pods here.
	log.Infoln("Shutting down.")

	select {
	case sig = <-signalChan:
		// If there is an interrupt signal, interrupt the run.
		log.Warnln("Received a second interrupt signal from the signal channel.")
		log.Debugln("Signal received was:", sig.String())
	case <-time.After(time.Duration(time.Second * 30)):
		// Exit if the clean up took to long to provide a response.
		log.Infoln("Clean up took too long to complete and timed out.")
	}

	os.Exit(0)
}
