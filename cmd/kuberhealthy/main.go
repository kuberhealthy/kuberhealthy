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

// Kuberhealthy is an enhanced health check for Kubernetes clusters.
package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/integrii/flaggy"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	khjobcrd "github.com/Comcast/kuberhealthy/v2/pkg/apis/khjob/v1"
	"github.com/Comcast/kuberhealthy/v2/pkg/khcheckcrd"
	"github.com/Comcast/kuberhealthy/v2/pkg/khstatecrd"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
	"github.com/Comcast/kuberhealthy/v2/pkg/masterCalculation"
)

// status represents the current Kuberhealthy OK:Error state
var cfg *Config
var configPath = "/etc/config/kuberhealthy.yaml"

var configPathDir = "/etc/config/"
var podCheckNamespaces = "kube-system"
var podNamespace = os.Getenv("POD_NAMESPACE")
var isMaster bool                  // indicates this instance is the master and should be running checks
var upcomingMasterState bool       // the upcoming master state on next interval
var lastMasterChangeTime time.Time // indicates the last time a master change was seen
var listenNamespace string         // namespace to listen (watch/get) `khcheck` resources on.  If blank, all namespaces will be monitored.

var terminationGracePeriod = time.Minute * 5 // keep calibrated with kubernetes terminationGracePeriodSeconds

// the hostname of this pod
var podHostname string
var enablePodStatusChecks = determineCheckStateFromEnvVar("POD_STATUS_CHECK")
var enableExternalChecks = true

// KHExternalReportingURL is the environment variable key used to override the URL checks will be asked to report in to
const KHExternalReportingURL = "KH_EXTERNAL_REPORTING_URL"

// DefaultRunInterval is the default run interval for checks set by kuberhealthy
const DefaultRunInterval = time.Minute * 10

// KHCheckNameAnnotationKey is the key used in the annotation that holds the check's short name
const KHCheckNameAnnotationKey = "comcast.github.io/check-name"

// var externalCheckReportingURL = os.Getenv(KHExternalReportingURL)
var khStateClient *khstatecrd.KuberhealthyStateClient

// constants for using the kuberhealthy status CRD
const stateCRDGroup = "comcast.github.io"
const stateCRDVersion = "v1"
const stateCRDResource = "khstates"

var khCheckClient *khcheckcrd.KuberhealthyCheckClient

// constants for using the kuberhealthy check CRD
const checkCRDGroup = "comcast.github.io"
const checkCRDVersion = "v1"
const checkCRDResource = "khchecks"

// instantiate kuberhealthy job client CRD
var khJobClient *khjobcrd.KHJobV1Client

// the global kubernetes client
var kubernetesClient *kubernetes.Clientset

func init() {

	cfg = &Config{
		kubeConfigFile: filepath.Join(os.Getenv("HOME"), ".kube", "config"),
		LogLevel:       "info",
	}

	var useDebugMode bool

	// setup flaggy
	flaggy.SetDescription("Kuberhealthy is an in-cluster synthetic health checker for Kubernetes.")
	flaggy.String(&configPath, "c", "config", "(optional) absolute path to the kuberhealthy config file")
	flaggy.Bool(&useDebugMode, "d", "debug", "Set to true to enable debug.")
	flaggy.Parse()

	// attempt to load config file from disk
	err := cfg.Load(configPath)
	if err != nil {
		log.Println("WARNING: Failed to read configuration file from disk:", err)
	}

	// set env variables into config if specified. otherwise set external check URL to default
	externalCheckURL, err := getEnvVar(KHExternalReportingURL)
	if err != nil {
		if len(podNamespace) == 0 {
			log.Fatalln("KH_EXTERNAL_REPORTING_URL environment variable not set and POD_NAMESPACE environment variable was blank.  Could not determine Kuberhealthy callback URL.")
		}
		log.Infoln("KH_EXTERNAL_REPORTING_URL environment variable not set, using default value")
		externalCheckURL = "http://kuberhealthy." + podNamespace + ".svc.cluster.local/externalCheckStatus"
	}
	cfg.ExternalCheckReportingURL = externalCheckURL
	log.Infoln("External check reporting URL set to:", cfg.ExternalCheckReportingURL)

	// parse and set logging level
	parsedLogLevel, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatalln("Unable to parse log-level flag: ", err)
	}

	// log to stdout and set the level to info by default
	log.SetOutput(os.Stdout)
	log.SetLevel(parsedLogLevel)
	log.Infoln("Startup Arguments:", os.Args)

	// no matter what if user has specified debug leveling, use debug leveling
	if useDebugMode {
		log.Infoln("Setting debug output on because user specified flag")
		log.SetLevel(log.DebugLevel)
	}

	// Handle force master mode
	if cfg.EnableForceMaster == true {
		log.Infoln("Enabling forced master mode")
		masterCalculation.DebugAlwaysMasterOn()
	}

	// determine the name of this pod from the POD_NAME environment variable
	podHostname, err = getEnvVar("POD_NAME")
	if err != nil {
		log.Fatalln("Failed to determine my hostname!")
	}

	// setup all clients
	err = initKubernetesClients()
	if err != nil {
		log.Fatalln("Failed to bootstrap kubernetes clients:", err)
	}
}

func main() {

	// Create a new Kuberhealthy struct
	kuberhealthy := NewKuberhealthy()
	kuberhealthy.ListenAddr = cfg.ListenAddress

	// create run context and start listening for shutdown interrupts
	khRunCtx, khRunCtxCancelFunc := context.WithCancel(context.Background())
	kuberhealthy.shutdownCtxFunc = khRunCtxCancelFunc // load the KH struct with a func to shutdown its control system
	go listenForInterrupts(kuberhealthy)

	// tell Kuberhealthy to restart if configmap has been changed
	go configReloader(kuberhealthy)

	// tell Kuberhealthy to start all checks and master change monitoring
	kuberhealthy.Start(khRunCtx)

	time.Sleep(time.Second * 90) // give the interrupt handler a period of time to call exit before we shutdown
	<-time.After(terminationGracePeriod + (time.Second * 10))
	log.Errorln("shutdown: main loop was ready for shutdown for too long. exiting.")
	os.Exit(1)
}

// listenForInterrupts watches for termination signals and acts on them
func listenForInterrupts(k *Kuberhealthy) {
	// shutdown signal handling
	sigChan := make(chan os.Signal, 1)

	// register for shutdown events on sigChan
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)
	log.Infoln("shutdown: waiting for sigChan notification...")
	<-sigChan
	log.Infoln("shutdown: Shutting down due to sigChan signal...")

	// wait for check to fully shutdown before exiting
	doneChan := make(chan struct{})
	go k.Shutdown(doneChan)

	// wait for checks to be done shutting down before exiting
	select {
	case <-doneChan:
		log.Infoln("shutdown: Shutdown gracefully completed!")
		log.Infoln("shutdown: exiting 0")
		os.Exit(0)
	case <-sigChan:
		log.Warningln("shutdown: Shutdown forced from multiple interrupts!")
		log.Infoln("shutdown: exiting 1")
		os.Exit(1)
	case <-time.After(terminationGracePeriod):
		log.Errorln("shutdown: Shutdown took too long.  Shutting down forcefully!")
		log.Infoln("shutdown: exiting 1")
		os.Exit(1)
	}
}

// determineCheckStateFromEnvVar determines a check's enabled state based on
// the supplied environment variable
func determineCheckStateFromEnvVar(envVarName string) bool {
	enabledState, err := strconv.ParseBool(os.Getenv(envVarName))
	if err != nil {
		log.Debugln("Had an error parsing the environment variable", envVarName, err)
		return true // by default, the check is on
	}
	return enabledState
}

// initKubernetesClients creates the appropriate CRD clients and kubernetes client to be used in all cases. Issue #181
func initKubernetesClients() error {

	// make a new kuberhealthy client
	kc, err := kubeClient.Create(cfg.kubeConfigFile)
	if err != nil {
		return err
	}
	kubernetesClient = kc

	// make a new crd check client
	checkClient, err := khcheckcrd.Client(checkCRDGroup, checkCRDVersion, cfg.kubeConfigFile, "")
	if err != nil {
		return err
	}
	khCheckClient = checkClient

	// make a new crd state client
	stateClient, err := khstatecrd.Client(stateCRDGroup, stateCRDVersion, cfg.kubeConfigFile, "")
	if err != nil {
		return err
	}
	khStateClient = stateClient

	// make a new crd job client
	jobClient, err := khjobcrd.Client(cfg.kubeConfigFile)
	if err != nil {
		return err
	}
	khJobClient = jobClient

	return nil
}
