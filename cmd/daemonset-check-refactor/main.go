package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
)

var (
	// K8s config file for the client
	kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

	// Namespace the check daemonset will be created in [default = kuberhealthy]
	checkNamespaceEnv = os.Getenv("POD_NAMESPACE")
	checkNamespace    string

	// DSPauseContainerImageOverride specifies the sleep image we will use on the daemonset checker
	dsPauseContainerImageEnv = os.Getenv("PAUSE_CONTAINER_IMAGE")
	dsPauseContainerImage string // specify an alternate location for the DSC pause container - see #114

	// Minutes allowed for the shutdown process to complete
	shutdownGracePeriodEnv = os.Getenv("SHUTDOWN_GRACE_PERIOD")
	shutdownGracePeriod    time.Duration

	// Check daemonset name
	checkDSNameEnv = os.Getenv("CHECK_DAEMONSET_NAME")
	checkDSName    string

	// Check time limit from injected env variable KH_CHECK_RUN_DEADLINE
	checkTimeLimit time.Duration

	// Daemonset check configurations
	hostName string
	tolerations []apiv1.Toleration
	daemonSetName string
	daemonsetDeployed bool
	shuttingDown bool
	daemonSet *appsv1.DaemonSet

	// Time object used for the check.
	now time.Time

	// Run context
	ctx       context.Context
	ctxCancel context.CancelFunc

	// Cleanup context
	cleanUpCtx       context.Context
	cleanUpCtxCancel context.CancelFunc

	// Interrupt signal channels.
	signalChan 			chan os.Signal
	doneChan   			chan error
	interruptSignal     os.Signal

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
	// Default daemonset check time limit
	defaultCheckTimeLimit = time.Duration(time.Minute * 15)
	// Default user
	defaultUser = int64(1000)
)

func init() {

	// Parse all incoming input environment variables and crash if an error occurs
	// during parsing process.
	parseInputValues()

	// Create a timestamp reference for the daemonset;
	// also to reference against daemonsets that should be cleaned up.
	now = time.Now()
	setCheckConfigurations(now)

	// Allocate channels.
	signalChan = make(chan os.Signal, 5)
	doneChan = make(chan error, 5)
}

func main() {

	log.Debugln("Allowing this check", checkTimeLimit, "to finish.")
	ctx, ctxCancel = context.WithTimeout(context.Background(), checkTimeLimit)

	// Create a kubernetes client.
	var err error
	client, err = kubeClient.Create(kubeConfigFile)
	if err != nil {
		errorMessage := "failed to create a kubernetes client with error: " + err.Error()
		reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + errorMessage})
		return
	}
	log.Infoln("Kubernetes client created.")

	// Start listening to interrupts.
	interruptCtx, interruptCtxCancel := context.WithTimeout(context.Background(), checkTimeLimit)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		listenForInterrupts(interruptCtx, interruptCtxCancel)
	}()

	// Catch panics.
	var r interface{}
	defer func() {
		r = recover()
		if r != nil {
			log.Infoln("Recovered panic:", r)
			reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + r.(string)})
		}
	}()

	// Run daemonset check.
	runCheck()
	// Check if interrupt signal was received, and if so, wait to finish shutdown before exiting.
	if interruptSignal != nil {
		wg.Wait()
	}
	log.Infoln("Done running daemonset check")
}


// setCheckConfigurations sets Daemonset configurations
func setCheckConfigurations(now time.Time) {
	hostName = getHostname()
	daemonSetName = checkDSName + "-" + hostName + "-" + strconv.Itoa(int(now.Unix()))
}

// waitForShutdown watches the signal and done channels for termination.
func listenForInterrupts(interruptCtx context.Context, interruptCtxCancel context.CancelFunc) {

	interruptSignal = nil
	// Relay incoming OS interrupt signals to the signalChan
	signal.Notify(signalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)
	interruptSignal =<-signalChan
	log.Infoln("Received an interrupt signal from the signal channel.")
	log.Debugln("Signal received was:", interruptSignal.String())

	doneChan <- shutdown(interruptCtx, interruptCtxCancel)

	select {
	case err := <-doneChan:
		if err != nil {
			log.Errorln("Error waiting for pod removal during shut down:", err)
			os.Exit(1)
		}
		log.Infoln("Shutdown gracefully completed!")
	case sig := <-signalChan:
		log.Warningln("Shutdown forced from multiple interrupts. Received signal:", sig.String())
		os.Exit(1)
	case <-time.After(time.Duration(shutdownGracePeriod)):
		log.Errorln("Shutdown took too long. Shutting down forcefully!")
		os.Exit(2)
	}
	os.Exit(0)
}

// Shutdown signals the DS to begin a cleanup
func shutdown(interruptCtx context.Context, interruptCtxCancel context.CancelFunc) error {

	log.Debugln("Cancelling context.")
	ctxCancel() // Causes all functions within the check to return without error and abort. NOT an error
	// condition; this is a response to an external shutdown signal.
	log.Infoln("Shutting down due to interrupt.")
	shuttingDown = true

	var err error
	outChan := make(chan error, 10)
	// if the ds is deployed, delete it
	if daemonsetDeployed {

		log.Infoln("Removing daemonset due to shutdown.")

		err = deleteDS(daemonSetName)
		if err != nil {
			log.Errorln("Failed to remove", daemonSetName)
			return err
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			log.Debugln("Worker: waitForPodRemoval started")
			defer wg.Done()
			outChan <- waitForPodRemoval(interruptCtx)
			wg.Wait()
		}()

		select {
		case err = <-outChan:
			if err != nil {
				interruptCtxCancel() // cancel the watch context, we have timed out
				log.Errorln("Error waiting for daemonset pods removal:", err)
				reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + err.Error()})
				return err
			}
			log.Infoln("Successfully removed daemonset pods.")
		case <- time.After(checkTimeLimit):
			errorMessage := "Reached check pod timeout: " + checkTimeLimit.String() + " waiting for daemonset and daemonset pods removal."
			log.Errorln(errorMessage)
			reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + errorMessage})
			return errors.New(errorMessage)
		case <-interruptCtx.Done():
			// If there is a cancellation interrupt signal.
			log.Infoln("Canceling removing daemonset and daemonset pods and shutting down from interrupt.")
			return err
		}
	}

	reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: Failed to complete check due to an interrupt signal."})
	log.Infoln("Daemonset", daemonSetName, "ready for shutdown.")
	return err
}

// getDSClient returns a daemonset client, useful for interacting with daemonsets
func getDSClient() v1.DaemonSetInterface {
	log.Debug("Creating Daemonset client.")
	return client.AppsV1().DaemonSets(checkNamespace)
}

// getPodClient returns a pod client, useful for interacting with pods
func getPodClient() v12.PodInterface {
	log.Debug("Creating Pod client.")
	return client.CoreV1().Pods(checkNamespace)
}

// getNodeClient returns a node client, useful for interacting with nodes
func getNodeClient() v12.NodeInterface {
	log.Debug("Creating Node client.")
	return client.CoreV1().Nodes()
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
