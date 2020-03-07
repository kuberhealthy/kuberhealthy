package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/prometheus/common/log"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultBlacklistNamespaces = []string{"default"}

	defaultWhitelistNamespaces = []string{"kube-system", "kuberhealthy"}

	defaultCheckTimeLimit = time.Minute * 5
)

var (
	// K8s config file for the client.
	kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

	// Ignores namespaces from the blacklist if turned on. (On by default, checks all namespaces)
	blacklistOnEnv = os.Getenv("BLACKLIST_ON")
	blacklistOn    = true

	// Looks at specified namespaces from the whitelist if turned on.
	whitelistOnEnv = os.Getenv("WHITELIST_ON")
	whitelistOn    bool

	// Given namespaces to ignore to look at. (Determined by BLACKLIST_ON or WHITELIST_ON environment variables.)
	// Blacklist is enabled by default (looks at all namespaces).
	// Expects a comma separated list of values (i.e. "default,kube-system")
	namespacesEnv = os.Getenv("NAMESPACES")
	namespaces    []string

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
