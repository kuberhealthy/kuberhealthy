package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
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
	dsPauseContainerImage    string // specify an alternate location for the DSC pause container - see #114

	// Minutes allowed for the shutdown process to complete
	shutdownGracePeriodEnv = os.Getenv("SHUTDOWN_GRACE_PERIOD")
	shutdownGracePeriod    time.Duration

	// Check daemonset name
	checkDSNameEnv = os.Getenv("CHECK_DAEMONSET_NAME")
	checkDSName    string

	// Check time limit from injected env variable KH_CHECK_RUN_DEADLINE
	checkTimeLimit time.Duration

	// Daemonset check configurations
	hostName      string
	tolerations   []apiv1.Toleration
	daemonSetName string
	daemonSet     *appsv1.DaemonSet

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

}

func main() {
	// Catch panics.
	var r interface{}
	defer func() {
		r = recover()
		if r != nil {
			log.Infoln("Recovered panic:", r)
			reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + r.(string)})
		}
	}()

	// create a context for our check to operate on that represents the timelimit the check has
	log.Debugln("Allowing this check", checkTimeLimit, "to finish.")
	ctx, ctxCancel := context.WithTimeout(context.Background(), checkTimeLimit)

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
	go listenForInterrupts(ctxCancel)

	// Run daemonset check.
	err = runCheck(ctx)
	log.Infoln("Done running daemonset check")
	if err != nil {
		err := checkclient.ReportFailure([]string{err.Error()})
		if err != nil {
			log.Fatalln("Was unable to report error to Kuberhealthy")
		}
		return
	}
	err = checkclient.ReportSuccess()
	if err != nil {
		log.Fatalln("Was unable to report success to Kuberhealthy")
	}
}

// setCheckConfigurations sets Daemonset configurations
func setCheckConfigurations(now time.Time) {
	hostName = getHostname()
	daemonSetName = checkDSName + "-" + hostName + "-" + strconv.Itoa(int(now.Unix()))
}

// waitForShutdown watches the signal and done channels for termination.
func listenForInterrupts(ctxCancel context.CancelFunc) {

	signalChan := make(chan os.Signal, 5)

	// Relay incoming OS interrupt signals to the signalChan
	signal.Notify(signalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	// watch for interrupts on signalChan
	<-signalChan
	ctxCancel() // Causes all functions within the check to return without error and abort. NOT an error

	// start a shutdown in the background
	shutdownCompleteChan := make(chan error)
	go func() {
		shutdownCompleteChan <- shutdown()
	}()

	// watch for timeout or shutdown to complete
	select {
	case err := <-shutdownCompleteChan:
		if err != nil {
			log.Infoln("shutdown completed with error:", err)
			os.Exit(1)
		}
		log.Infoln("shutdown completed without error")
		os.Exit(0)
	case <-time.After(time.Duration(shutdownGracePeriod)):
		log.Errorln("Shutdown took too long. Shutting down forcefully!")
		os.Exit(2)
	}
	os.Exit(0)
}

// Shutdown signals the DS to begin a cleanup
func shutdown() error {

	// condition; this is a response to an external shutdown signal.
	log.Infoln("Shutting down... ")

	// if the ds is deployed, delete it
	if daemonsetDeployed {

		log.Infoln("Removing daemonset due to shutdown.")
		err := deleteDS(daemonSetName)
		if err != nil {
			log.Errorln("Failed to remove", daemonSetName)
		}

		// wait for pod removal
		err = waitForPodRemoval()
		if err != nil {
			log.Errorln("error when waiting for daemonset pods to be removed for daemonset", daemonSetName)
		}

		log.Infoln("Daemonset", daemonSetName, "ready for shutdown.")
	}

	return nil
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
