package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	kh "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
)

var (
	// K8s config file for the client
	kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

	// Namespace the check daemonset will be created in [default = kuberhealthy]
	checkNamespaceEnv = os.Getenv("POD_NAMESPACE")
	checkNamespace    string

	// DSPauseContainerImageOverride specifies the sleep image we will use on the daemonset checker
	dsPauseContainerImageEnv = os.Getenv("PAUSE_CONTAINER_IMAGE")
	dsPauseContainerImage    string // specify an alternate location for the DSC pause container - see #114

	// Node selectors for the daemonset check
	dsNodeSelectorsEnv = os.Getenv("NODE_SELECTOR")
	dsNodeSelectors    = make(map[string]string)

	// Minutes allowed for the shutdown process to complete
	shutdownGracePeriodEnv = os.Getenv("SHUTDOWN_GRACE_PERIOD")
	shutdownGracePeriod    time.Duration

	// Check daemonset name
	checkDSNameEnv = os.Getenv("CHECK_DAEMONSET_NAME")
	checkDSName    string

	// The priority class to use for the daemonset
	podPriorityClassNameEnv = os.Getenv("DAEMONSET_PRIORITY_CLASS_NAME")
	podPriorityClassName    string

	// Check deadline from injected env variable KH_CHECK_RUN_DEADLINE
	khDeadline    time.Time
	checkDeadline time.Time

	// Daemonset check configurations
	hostName         string
	tolerationsEnv   = os.Getenv("TOLERATIONS")
	tolerations      []apiv1.Toleration
	daemonSetName    string
	allowedTaintsEnv = os.Getenv("ALLOWED_TAINTS")
	allowedTaints    map[string]apiv1.TaintEffect

	// Time object used for the check.
	now time.Time

	// K8s client used for the check.
	client *kubernetes.Clientset
)

const (
	// Default k8s manifest resource names.
	defaultCheckDSName = "daemonset"
	// Default namespace daemonset check will be performed in
	defaultCheckNamespace = "kuberhealthy"
	// Default pause container image used for the daemonset check
	defaultDSPauseContainerImage = "gcr.io/google-containers/pause:3.1"
	// Default shutdown termination grace period
	defaultShutdownGracePeriod = time.Duration(time.Minute * 1) // grace period for the check to shutdown after receiving a shutdown signal
	// Default daemonset check deadline
	defaultCheckDeadline = time.Duration(time.Minute * 15)
	// Default user
	defaultUser = int64(1000)
	// Default priority class name
	defaultPodPriorityClassName = ""
)

func init() {
	// set debug mode for nodeCheck pkg
	nodeCheck.EnableDebugOutput()

	// Create a timestamp reference for the daemonset;
	// also to reference against daemonsets that should be cleaned up.
	now = time.Now()

	// Parse all incoming input environment variables and crash if an error occurs
	// during parsing process.
	parseInputValues()

	setCheckConfigurations(now)
}

func main() {
	// Create a kubernetes client.
	var err error
	client, err = kubeClient.Create(kubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client:" + err.Error())
	}
	log.Infoln("Kubernetes client created.")

	// this check runs all the nodechecks to ensure node is ready before running the daemonset chek
	err = checksNodeReady()
	if err != nil {
		log.Errorln("Error running when doing the nodechecks :", err)
	}

	// Catch panics.
	defer func() {
		r := recover()
		if r != nil {
			log.Infoln("Recovered panic:", r)
			reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + r.(string)})
		}
	}()

	// create a context for our check to operate on that represents the timelimit the check has
	log.Debugln("Allowing this check until", checkDeadline, "to finish.")
	// Set ctx and ctxChancel using khDeadline. If timeout is set to checkDeadline, ctxCancel will happen first before
	// any of the timeouts are given the chance to report their timeout errors.
	log.Debugln("Setting check ctx cancel with timeout", khDeadline.Sub(now))
	ctx, ctxCancel := context.WithTimeout(context.Background(), khDeadline.Sub(now))

	// Start listening to interrupts.
	signalChan := make(chan os.Signal, 5)
	go listenForInterrupts(signalChan)

	// run check in background and wait for completion
	runCheckDoneChan := make(chan error, 1)
	go func() {
		// Run daemonset check and report errors
		runCheckDoneChan <- runCheck(ctx)
	}()

	// watch for either the check to complete or the OS to get a shutdown signal
	select {
	case err = <-runCheckDoneChan:
		if err != nil {
			reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + err.Error()})
		} else {
			reportOKToKuberhealthy()
		}
		log.Infoln("Done running daemonset check")
	case <-signalChan:
		// TO DO: figure out better way to report shutdown signals. Do we report "error" or "ok" to kuberhealthy when
		// a shutdown signal is received? For now, report OK and wait for the next run.
		reportOKToKuberhealthy()
		log.Errorln("Received shutdown signal. Canceling context and proceeding directly to cleanup.")
		ctxCancel() // Causes all functions within the check to return without error and abort. NOT an error
	}

	// at the end of the check run, we run a clean up for everything that may be left behind
	log.Infoln("Running post-check cleanup")
	shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer shutdownCtxCancel()

	// start a background cleanup
	cleanupDoneChan := make(chan error)
	go func() {
		cleanupDoneChan <- cleanUp(shutdownCtx)
	}()

	// wait for either the cleanup to complete or the shutdown grace period to expire
	select {
	case err := <-cleanupDoneChan:
		if err != nil {
			log.Errorln("Cleanup completed with error:", err)
			return
		}
		log.Infoln("Cleanup completed without error")
	case <-time.After(time.Duration(shutdownGracePeriod)):
		log.Errorln("Shutdown took too long. Shutting down forcefully!")
	}
}

// checksNodeReady checks whether node is ready or not before running the check
func checksNodeReady() error {
	// create context
	checkTimeLimit := time.Minute * 1
	nctx, _ := context.WithTimeout(context.Background(), checkTimeLimit)

	// hits kuberhealthy endpoint to see if node is ready
	err := nodeCheck.WaitForKuberhealthy(nctx)
	if err != nil {
		log.Errorln("Error waiting for kuberhealthy endpoint to be contactable by checker pod with error:" + err.Error())
	}

	return nil
}

// setCheckConfigurations sets Daemonset configurations
func setCheckConfigurations(now time.Time) {
	hostName = getHostname()
	daemonSetName = checkDSName + "-" + hostName + "-" + strconv.Itoa(int(now.Unix()))
}

// waitForShutdown watches the signal and done channels for termination.
func listenForInterrupts(signalChan chan os.Signal) {

	// Relay incoming OS interrupt signals to the signalChan
	signal.Notify(signalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	// watch for interrupts on signalChan
	<-signalChan
	os.Exit(0)
}

// reportErrorsToKuberhealthy reports the specified errors for this check run.
func reportErrorsToKuberhealthy(errs []string) {
	log.Errorln("Reporting errors to Kuberhealthy:", errs)
	reportToKuberhealthy(false, errs)
}

// reportOKToKuberhealthy reports that there were no errors on this check run to Kuberhealthy.
func reportOKToKuberhealthy() {
	log.Infoln("Reporting success to Kuberhealthy.")
	reportToKuberhealthy(true, []string{})
}

// reportToKuberhealthy reports the check status to Kuberhealthy.
func reportToKuberhealthy(ok bool, errs []string) {
	var err error
	if ok {
		err = kh.ReportSuccess()
		if err != nil {
			log.Fatalln("error reporting to kuberhealthy:", err.Error())
		}
		return
	}
	err = kh.ReportFailure(errs)
	if err != nil {
		log.Fatalln("error reporting to kuberhealthy:", err.Error())
	}
	return
}
