// Copyright 2018 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	kh "github.com/Comcast/kuberhealthy/pkg/checks/external/checkClient"
	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
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

	// Check time limit.
	checkTimeLimitSecondsEnv = os.Getenv("CHECK_TIME_LIMIT_SECONDS")
	checkTimeLimit           time.Duration

	// Boolean value if a rolling-update is requested.
	rollingUpdateEnv = os.Getenv("CHECK_DEPLOYMENT_ROLLING_UPDATE")
	rollingUpdate    bool

	// Additional container environment variables if a custom image is used for the deployment.
	additionalEnvVarsEnv = os.Getenv("ADDITIONAL_ENV_VARS")
	additionalEnvVars    = make(map[string]string, 0)

	// Seconds allowed for the shutdown process to complete.
	shutdownGracePeriodSecondsEnv = os.Getenv("SHUTDOWN_GRACE_PERIOD_SECONDS")
	shutdownGracePeriodSeconds    int

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
	defaultCheckContainerName = "kh-deployment-check-container"

	// Default images used for check.
	defaultCheckImageURL  = "nginx:latest"
	defaultCheckImageURLB = "nginx:alpine"

	// Default container port used for check.
	defaultCheckContainerPort = int32(80)

	// Default load balancer port used for check.
	defaultCheckLoadBalancerPort = int32(80)

	// Default k8s manifest resource names.
	defaultCheckDeploymentName = "kh-deployment-check-deployment"
	defaultCheckServiceName    = "kh-deployment-check-service"

	// Default namespace for the check to run in.
	defaultCheckNamespace = "kuberhealthy"

	// Default number of replicas the deployment should bring up.
	defaultCheckDeploymentReplicas = 2

	defaultCheckTimeLimit             = time.Duration(time.Minute * 5)
	defaultShutdownGracePeriodSeconds = 30 // grace period for the check to shutdown after receiving a shutdown signal
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
	go listenForInterrupts()

	// Start deployment check.
	err = runDeploymentCheck()
	if err != nil {
		log.Errorln("Error when running check:", err)
		os.Exit(1)
	}
}

// listenForInterrupts watches the signal and done channels for termination.
func listenForInterrupts() {

	// Relay incoming OS interrupt signals to the signalChan.
	signal.Notify(signalChan, os.Interrupt, os.Kill)
	<-signalChan // This is a blocking operation -- the routine will stop here until there is something sent down the channel.
	log.Infoln("Received an interrupt signal from the signal channel.")

	log.Debugln("Cancelling context.")
	ctxCancel() // Causes all functions within the check to return without error and abort. NOT an error
	// condition; this is a response to an external shutdown signal.

	// Clean up pods here.
	log.Infoln("Shutting down.")

	select {
	case <-signalChan:
		// If there is an interrupt signal, interrupt the run.
		log.Warnln("Received a secsond interrupt signal from the signal channel.")
	case err := <-cleanUpAndWait():
		// If the clean up is complete, exit.
		log.Infoln("Received a complete signal, clean up completed.")
		if err != nil {
			log.Errorln("failed to clean up check resources properly:", err.Error())
		}
	case <-time.After(time.Duration(shutdownGracePeriodSeconds)):
		// Exit if the clean up took to long to provide a response.
		log.Infoln("Clean up took too long to complete and timed out.")
	}

	os.Exit(0)
}

// cleanUpAndWait cleans up things and returns a signal down the returned channel when completed.
func cleanUpAndWait() chan error {

	// Watch for the clean up process to complete.
	doneChan := make(chan error)
	go func() {
		doneChan <- cleanUp()
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
	var attempts int
	var err error

	retry := func() {
		log.Infoln("Retrying a report to Kuberhealthy in 5 seconds.")
		time.Sleep(time.Second * 5)
	}

	// Keep retrying until it works.
	for {
		attempts++
		log.Infoln("Reporting status to Kuberhealthy:", ok)

		if attempts > 1 {
			log.Infoln("Attempt", attempts, "reporting status to Kuberhealthy.")
		}

		if ok {
			err = kh.ReportSuccess()
			if err != nil {
				retry()
				continue
			}
			return
		}
		err = kh.ReportFailure(errs)
		if err != nil {
			retry()
			continue
		}
		return
	}
}
