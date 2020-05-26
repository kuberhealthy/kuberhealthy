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

	"github.com/Comcast/kuberhealthy/v2/pkg/khcheckcrd"
	"github.com/Comcast/kuberhealthy/v2/pkg/khstatecrd"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
	"github.com/Comcast/kuberhealthy/v2/pkg/masterCalculation"
)

// status represents the current Kuberhealthy OK:Error state
var cfg *Config
var configPath = "/etc/config/kuberhealthy.yaml"
var podCheckNamespaces = "kube-system"
var podNamespace = os.Getenv("POD_NAMESPACE")
var isMaster bool                  // indicates this instance is the master and should be running checks
var upcomingMasterState bool       // the upcoming master state on next interval
var lastMasterChangeTime time.Time // indicates the last time a master change was seen

var terminationGracePeriod = time.Minute * 5 // keep calibrated with kubernetes terminationGracePeriodSeconds

// flags indicating that checks of specific types should be used
var DSPauseContainerImageOverride string // specify an alternate location for the DSC pause container - see #114
// DSTolerationOverride specifies an alternate list of taints to tolerate - see #178
var DSTolerationOverride []string

// the hostname of this pod
var podHostname string
var enablePodStatusChecks = determineCheckStateFromEnvVar("POD_STATUS_CHECK")
var enableExternalChecks = true

// external check configs
const KHExternalReportingURL = "KH_EXTERNAL_REPORTING_URL"

// default run interval set by kuberhealthy
const DefaultRunInterval = time.Minute * 10

// the key used in the annotation that holds the check's short name
const KH_CHECK_NAME_ANNOTATION_KEY = "comcast.github.io/check-name"

// var externalCheckReportingURL = os.Getenv(KHExternalReportingURL)

var kuberhealthy *Kuberhealthy

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

	// set env variables into config if specified
	externalCheckURL, err := getEnvVar(KHExternalReportingURL)
	if err != nil {
		cfg.ExternalCheckReportingURL = externalCheckURL
	}

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

	// parse external check URL configuration
	if len(cfg.ExternalCheckReportingURL) == 0 {
		if len(podNamespace) == 0 {
			log.Fatalln("KH_EXTERNAL_REPORTING_URL environment variable not set and POD_NAMESPACE environment variable was blank.  Could not determine Kuberhealthy callback URL.")
		}
		cfg.ExternalCheckReportingURL = "http://kuberhealthy." + podNamespace + ".svc.cluster.local/externalCheckStatus"
	}
	log.Infoln("External check reporting URL set to:", cfg.ExternalCheckReportingURL)

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
	kuberhealthy = NewKuberhealthy()
	kuberhealthy.ListenAddr = cfg.ListenAddress

	// tell Kuberhealthy to restart if configmap has been changed
	go configReloader(kuberhealthy)

	// create run context and start listening for shutdown interrupts
	khRunCtx, khRunCtxCancelFunc := context.WithCancel(context.Background())
	kuberhealthy.shutdownCtxFunc = khRunCtxCancelFunc // load the KH struct with a func to shutdown its control system
	go listenForInterrupts()

	// tell Kuberhealthy to start all checks and master change monitoring
	kuberhealthy.Start(khRunCtx)

	time.Sleep(time.Second * 90) // give the interrupt handler a period of time to call exit before we shutdown
	<-time.After(terminationGracePeriod + (time.Second * 10))
	log.Errorln("shutdown: main loop was ready for shutdown for too long. exiting.")
	os.Exit(1)
}

// listenForInterrupts watches for termination signals and acts on them
func listenForInterrupts() {
	// shutdown signal handling
	sigChan := make(chan os.Signal, 1)

	// register for shutdown events on sigChan
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)
	log.Infoln("shutdown: waiting for sigChan notification...")
	<-sigChan
	log.Infoln("shutdown: Shutting down due to sigChan signal...")

	// wait for check to fully shutdown before exiting
	doneChan := make(chan struct{})
	go kuberhealthy.Shutdown(doneChan)

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

	return nil
}

// configReloader watchers for events in file and restarts kuberhealhty checks
func configReloader(kh *Kuberhealthy) {
	fileChangedChan, err := fileChangeNotifier(configPath)
	if err != nil {
		log.Errorln("configReloader: Unable to watch config for changes:", err)
	}
	//TODO add logic for file notification spam with for loop
	for msg := range fileChangedChan {
		if msg.failed {
			log.Warningln("configReloader: Received error when watching for config to change:", msg.event, msg.path)
			continue
		}
		log.Infoln("configReloader: Restarting Kuberhealthy checks because configmap changed")

		// load new config and restart checks
		err := cfg.Load(configPath)
		if err != nil {
			log.Errorln("configReloader: Error reloading config:", err)
		}
		kh.RestartChecks()
	}
}
