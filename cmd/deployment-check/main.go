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
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	// K8s config file for the client.
	kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

	// Image the check deploy. If configured to do a rolling-update, this is the first
	// image used in the deployment.
	checkImageURLEnv = os.Getenv("CHECK_IMAGE")
	checkImageURL    string

	// Image the check will rolling-update to if configured to do a rolling update.
	checkImageURLBEnv = os.Getenv("CHECK_IMAGE_ROLL_TO")
	checkImageURLB    string

	// Image Pull Secret for check pods
	checkImagePullSecretEnv = os.Getenv("CHECK_IMAGE_PULL_SECRET")
	checkImagePullSecret    string

	// Deployment name that will be used for the check (in case an existing deployment uses a similar name).
	checkDeploymentNameEnv = os.Getenv("CHECK_DEPLOYMENT_NAME")
	checkDeploymentName    string

	// Service name that will be used for the check (in case an existing service uses a similar name).
	checkServiceNameEnv = os.Getenv("CHECK_SERVICE_NAME")
	checkServiceName    string

	// Container port that will be exposed for the deployment [default = 80] (for HTTP).
	checkContainerPortEnv = os.Getenv("CHECK_CONTAINER_PORT")
	checkContainerPort    int32

	// Load balancer port that will be exposed for the deployment [default = 80] (for HTTP).
	checkLoadBalancerPortEnv = os.Getenv("CHECK_LOAD_BALANCER_PORT")
	checkLoadBalancerPort    int32

	// Namespace the check deployment will be created in [default = kuberhealthy].
	checkNamespaceEnv = os.Getenv("CHECK_NAMESPACE")
	checkNamespace    string

	// Number of replicas the deployment will bring up [default = 2].
	checkDeploymentReplicasEnv = os.Getenv("CHECK_DEPLOYMENT_REPLICAS")
	checkDeploymentReplicas    int

	// Toleration values for the deployment check
	checkDeploymentTolerationsEnv = os.Getenv("TOLERATIONS")
	checkDeploymentTolerations    []apiv1.Toleration

	// Node selectors for the deployment check
	checkDeploymentNodeSelectorsEnv = os.Getenv("NODE_SELECTOR")
	checkDeploymentNodeSelectors    = make(map[string]string)

	// ServiceAccount that will deploy the test deployment [default = default]
	checkServiceAccountEnv = os.Getenv("CHECK_SERVICE_ACCOUNT")
	checkServiceAccount    string

	// Deployment pod resource requests and limits.
	millicoreRequestEnv = os.Getenv("CHECK_POD_CPU_REQUEST")
	millicoreRequest    int

	millicoreLimitEnv = os.Getenv("CHECK_POD_CPU_LIMIT")
	millicoreLimit    int

	memoryRequestEnv = os.Getenv("CHECK_POD_MEM_REQUEST")
	memoryRequest    int

	memoryLimitEnv = os.Getenv("CHECK_POD_MEM_LIMIT")
	memoryLimit    int

	// Check time limit.
	checkTimeLimit time.Duration

	// Boolean value if a rolling-update is requested.
	rollingUpdateEnv = os.Getenv("CHECK_DEPLOYMENT_ROLLING_UPDATE")
	rollingUpdate    bool

	// Additional container environment variables if a custom image is used for the deployment.
	additionalEnvVarsEnv = os.Getenv("ADDITIONAL_ENV_VARS")
	additionalEnvVars    = make(map[string]string)

	// Seconds allowed for the shutdown process to complete.
	shutdownGracePeriodEnv = os.Getenv("SHUTDOWN_GRACE_PERIOD")
	shutdownGracePeriod    time.Duration

	// Time object used for the check.
	now time.Time

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

const (
	// Default container name.
	defaultCheckContainerName = "deployment-container"

	// Default images used for check.
	defaultCheckImageURL  = "nginxinc/nginx-unprivileged:1.17.8"
	defaultCheckImageURLB = "nginxinc/nginx-unprivileged:1.17.9"

	// Default container port used for check.
	defaultCheckContainerPort = int32(8080)

	// Default load balancer port used for check.
	defaultCheckLoadBalancerPort = int32(80)

	// Default k8s manifest resource names.
	defaultCheckDeploymentName = "deployment-deployment"
	defaultCheckServiceName    = "deployment-svc"

	// Default k8s service account name.
	defaultCheckServieAccount = "default"

	// Default namespace for the check to run in.
	defaultCheckNamespace = "kuberhealthy"

	// Default number of replicas the deployment should bring up.
	defaultCheckDeploymentReplicas = 2

	defaultCheckTimeLimit      = time.Duration(time.Minute * 15)
	defaultShutdownGracePeriod = time.Duration(time.Second * 30) // grace period for the check to shutdown after receiving a shutdown signal
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

	// Create a timestamp reference for the deployment;
	// also to reference against deployments that should be cleaned up.
	now = time.Now()
	log.Debugln("Allowing this check", checkTimeLimit, "to finish.")
	ctx, ctxCancel = context.WithTimeout(context.Background(), checkTimeLimit)

	// Create a kubernetes client.
	var err error
	client, err = kubeClient.Create(kubeConfigFile)
	if err != nil {
		errorMessage := "failed to create a kubernetes client with error: " + err.Error()
		reportErrorsToKuberhealthy([]string{errorMessage})
		return
	}
	log.Infoln("Kubernetes client created.")

	// Start listening to interrupts.
	go listenForInterrupts(ctx, ctxCancel)

	// Catch panics.
	var r interface{}
	defer func() {
		r = recover()
		if r != nil {
			log.Infoln("Recovered panic:", r)
			reportToKuberhealthy(false, []string{r.(string)})
		}
	}()

	// Start deployment check.
	runDeploymentCheck(ctx)
}

// listenForInterrupts watches the signal and done channels for termination.
func listenForInterrupts(ctx context.Context, cancel context.CancelFunc) {

	// Relay incoming OS interrupt signals to the signalChan.
	signal.Notify(signalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)
	sig := <-signalChan // This is a blocking operation -- the routine will stop here until there is something sent down the channel.
	log.Infoln("Received an interrupt signal from the signal channel.")
	log.Debugln("Signal received was:", sig.String())

	log.Debugln("Cancelling context.")
	cancel() // Causes all functions within the check to return without error and abort. NOT an error
	// condition; this is a response to an external shutdown signal.

	// Clean up pods here.
	log.Infoln("Shutting down.")

	select {
	case sig = <-signalChan:
		// If there is an interrupt signal, interrupt the run.
		log.Warnln("Received a second interrupt signal from the signal channel.")
		log.Debugln("Signal received was:", sig.String())
	case err := <-cleanUpAndWait(ctx):
		// If the clean up is complete, exit.
		log.Infoln("Received a complete signal, clean up completed.")
		if err != nil {
			log.Errorln("failed to clean up check resources properly:", err.Error())
		}
	case <-time.After(time.Duration(shutdownGracePeriod)):
		// Exit if the clean up took to long to provide a response.
		log.Infoln("Clean up took too long to complete and timed out.")
	}

	os.Exit(0)
}

// cleanUpAndWait cleans up things and returns a signal down the returned channel when completed.
func cleanUpAndWait(ctx context.Context) chan error {

	// Watch for the clean up process to complete.
	doneChan := make(chan error)
	go func() {
		doneChan <- cleanUp(ctx)
	}()

	return doneChan
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
