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
	"syscall"
	"time"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

var (
	// K8s config file for the client.
	kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

	allowedCheckNodesEnv = os.Getenv("CHECK_STORAGE_ALLOWED_CHECK_NODES")
	ignoredCheckNodesEnv = os.Getenv("CHECK_STORAGE_IGNORED_CHECK_NODES")

	// By default, there is no storage class defined for the PVC (used the cluster default)
	storageClassNameEnv = os.Getenv("CHECK_STORAGE_PVC_STORAGE_CLASS_NAME")

	// Image for the storage check Job.
	checkStorageImageEnv = os.Getenv("CHECK_STORAGE_IMAGE")
	checkStorageImage    string

	// Image for the storage initialization for the PVC
	checkStorageInitImageEnv = os.Getenv("CHECK_STORAGE_INIT_IMAGE")
	checkStorageInitImage    string

	// Storage name that will be used for the PVC (in case an existing storage PVC uses a similar name).
	checkStorageNameEnv = os.Getenv("CHECK_STORAGE_NAME")
	checkStorageName    string

	// Storage Init job name that will be used for the initialization of the PVC (in case an existing Job uses a similar name).
	checkStorageInitJobNameEnv = os.Getenv("CHECK_STORAGE_INIT_JOB_NAME")
	checkStorageInitJobName    string

	// Storage Check job name that will be used for the check (in case an existing job uses a similar name).
	checkStorageJobNameEnv = os.Getenv("CHECK_STORAGE_JOB_NAME")
	checkStorageJobName    string

	// Check storage init command to give to the checkStorageInitImage container
	checkStorageInitCommandEnv = os.Getenv("CHECK_STORAGE_INIT_COMMAND")
	checkStorageInitCommand    string

	// Check storage init args to give to the checkStorageInitImage container command
	checkStorageInitCommandArgsEnv = os.Getenv("CHECK_STORAGE_INIT_COMMAND_ARGS")
	checkStorageInitCommandArgs    string

	// Check storage command to give to the checkStorageImage container
	checkStorageCommandEnv = os.Getenv("CHECK_STORAGE_COMMAND")
	checkStorageCommand    string

	// Check storage args to give to the checkStorageImage container command
	checkStorageCommandArgsEnv = os.Getenv("CHECK_STORAGE_COMMAND_ARGS")
	checkStorageCommandArgs    string

	// The size of the PVC Storage
	pvcSizeEnv = os.Getenv("CHECK_STORAGE_PVC_SIZE")
	pvcSize    string

	// Namespace the check deployment will be created in [default = kuberhealthy].
	checkNamespaceEnv = os.Getenv("CHECK_NAMESPACE")
	checkNamespace    string

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
	additionalEnvVars    = make(map[string]string, 0)

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

	// Default images used for check.
	defaultCheckStorageImage     = "alpine:3.11"
	defaultCheckStorageInitImage = "alpine:3.11"

	// Default k8s manifest resource names.
	defaultCheckStorageName        = "storage-check-pvc"
	defaultCheckStorageInitJobName = "storage-check-init-job"
	defaultCheckStorageJobName     = "storage-check-job"

	// The requested size of the PVC for the storage check
	defaultPvcSize = "1G"

	defaultCheckStorageInitCommand     = "/bin/sh"
	defaultCheckStorageInitCommandArgs = "echo storage-check-ok > /data/index.html && ls -la /data && cat /data/index.html"

	defaultCheckStorageCommand     = "/bin/sh"
	defaultCheckStorageCommandArgs = "ls -la /data && cat /data/index.html && cat /data/index.html | grep storage-check-ok"

	defaultCheckStoragePvcSize
	// Default namespace for the check to run in.
	defaultCheckNamespace = "kuberhealthy"

	defaultCheckTimeLimit      = time.Duration(time.Minute * 15)
	defaultShutdownGracePeriod = time.Duration(time.Second * 30) // grace period for the check to shutdown after receiving a shutdown signal

)

func init() {
	debug = true
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
	go listenForInterrupts(ctx)

	// Catch panics.
	var r interface{}
	defer func() {
		r = recover()
		if r != nil {
			log.Infoln("Recovered panic:", r)
			reportToKuberhealthy(false, []string{r.(string)})
		}
	}()

	// Start storage check.
	runStorageCheck()
}

// listenForInterrupts watches the signal and done channels for termination.
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
