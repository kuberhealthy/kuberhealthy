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
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/integrii/flaggy"
	log "github.com/sirupsen/logrus"

	"github.com/Comcast/kuberhealthy/pkg/masterCalculation"
)

// status represents the current Kuberhealthy OK:Error state
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var listenAddress = ":8080"
var podCheckNamespaces = "kube-system"
var dnsEndpoints []string
var podNamespace = os.Getenv("POD_NAMESPACE")
var isMaster bool // indicates this instance is the master and should be running checks

// shutdown signal handling
var sigChan chan os.Signal
var doneChan chan bool
var terminationGracePeriod = time.Minute * 5 // keep calibrated with kubernetes terminationGracePeriodSeconds

// flags indicating that checks of specific types should be used
var enableForceMaster bool // force master mode - for debugging
var enableDebug bool       // enable debug logging
// DSPauseContainerImageOverride specifies the sleep image we will use on the daemonset checker
var DSPauseContainerImageOverride string // specify an alternate location for the DSC pause container - see #114
// DSTolerationOverride specifies an alternate list of taints to tolerate - see #178
var DSTolerationOverride []string
var logLevel = "info"

var enableDaemonSetChecks = determineCheckStateFromEnvVar("DAEMON_SET_CHECK")
var enablePodRestartChecks = determineCheckStateFromEnvVar("POD_RESTARTS_CHECK")
var enablePodStatusChecks = determineCheckStateFromEnvVar("POD_STATUS_CHECK")
var enableDNSStatusChecks = determineCheckStateFromEnvVar("DNS_STATUS_CHECK")
var enableExternalChecks = true

// external check configs
const KHExternalReportingURL = "KH_EXTERNAL_REPORTING_URL"
var externalCheckReportingURL = os.Getenv(KHExternalReportingURL)

// InfluxDB connection configuration
var enableInflux = false
var influxURL = ""
var influxUsername = ""
var influxPassword = ""
var influxDB = "http://localhost:8086"
var kuberhealthy *Kuberhealthy

// constants for using the kuberhealthy status CRD
const statusCRDGroup = "comcast.github.io"
const statusCRDVersion = "v1"
const statusCRDResource = "khstates"

// constants for using the kuberhealthy check CRD
const checkCRDGroup = "comcast.github.io"
const checkCRDVersion = "v1"
const checkCRDResource = "khchecks"

var checkCRDScanInterval = time.Second * 5 // how often we scan for changes to check CRD objects

func init() {

	// setup flaggy
	flaggy.SetDescription("Kuberhealthy is an in-cluster synthetic health checker for Kubernetes.")
	flaggy.String(&kubeConfigFile, "", "kubecfg", "(optional) absolute path to the kubeconfig file")
	flaggy.String(&listenAddress, "l", "listenAddress", "The port for kuberhealthy to listen on for web requests")
	flaggy.Bool(&enableDaemonSetChecks, "", "daemonsetChecks", "Set to false to disable cluster daemonset deployment and termination checking.")
	flaggy.Bool(&enablePodRestartChecks, "", "podRestartChecks", "Set to false to disable pod restart checking.")
	flaggy.Bool(&enablePodStatusChecks, "", "podStatusChecks", "Set to false to disable pod lifecycle phase checking.")
	flaggy.Bool(&enableDNSStatusChecks, "", "dnsStatusChecks", "Set to false to disable DNS checks.")
	flaggy.Bool(&enableExternalChecks, "", "externalChecks", "Set to false to disable external checks.")
	flaggy.Bool(&enableForceMaster, "","forceMaster", "Set to true to enable local testing, forced master mode.")
	flaggy.Bool(&enableDebug, "d", "debug", "Set to true to enable debug.")
	flaggy.String(&DSPauseContainerImageOverride, "", "dsPauseContainerImageOverride", "Set an alternate image location for the pause container the daemon set checker uses for its daemon set configuration.")
	flaggy.StringSlice(&DSTolerationOverride, "", "tolerationOverride", "Specify a specific taint (in a key,value,effect format, ex. node-role.kubernetes.io/master,,NoSchedule or dedicated,someteam,NoSchedule)  to tolerate and force DaemonSetChecker to tolerate only nodes with that taint. Use the flag multiple times to add multiple tolerations. Default behavior is to tolerate all taints in the cluster.")
	flaggy.String(&podCheckNamespaces, "", "podCheckNamespaces", "The comma separated list of namespaces on which to check for pod status and restarts, if enabled.")
	flaggy.String(&logLevel, "", "log-level", fmt.Sprintf("Log level to be used one of [%s].", getAllLogLevel()))
	flaggy.StringSlice(&dnsEndpoints, "", "dnsEndpoints", "The comma separated list of dns endpoints to check, if enabled. Defaults to kubernetes.default")
	// Influx flags
	flaggy.String(&influxUsername, "", "influxUser", "Username for the InfluxDB instance")
	flaggy.String(&influxPassword, "", "influxPassword", "Password for the InfluxDB instance")
	flaggy.String(&influxURL, "", "influxUrl", "Address for the InfluxDB instance")
	flaggy.String(&influxDB, "", "influxDB", "Name of the InfluxDB database")
	flaggy.Bool(&enableInflux, "", "enableInflux", "Set to true to enable metric forwarding to Influx DB.")
	flaggy.Parse()

	parsedLogLevel, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatalln("Unable to parse log-level flag: ", err)
	}

	// log to stdout and set the level to info by default
	log.SetOutput(os.Stdout)
	log.SetLevel(parsedLogLevel)
	log.Infoln("Startup Arguments:", os.Args)

	// parse external check URL configuration
	if len(externalCheckReportingURL) == 0 {
		if len(podNamespace) == 0 {
			log.Fatalln("KH_EXTERNAL_REPORTING_URL environment variable not set and POD_NAMESPACE environment variable was blank.  Could not determine Kuberhealthy callback URL.")
		}
		externalCheckReportingURL = "http://kuberhealthy." + podNamespace + ".svc.cluster.local"
	}
	log.Infoln("External check reporting URL set to:", externalCheckReportingURL)

	// handle debug logging
	debugEnv := os.Getenv("DEBUG")
	if len(debugEnv) > 0 {
		enableDebug, err = strconv.ParseBool(debugEnv)
		if err != nil {
			log.Warningln("Failed to parse bool for DEBUG setting:",err)
		}
	}
	if enableDebug {
		log.Infoln("Enabling debug logging")
		log.SetLevel(log.DebugLevel)
		masterCalculation.EnableDebug()
	}

	// shutdown signal handling
	// we give a queue depth here to prevent blocking in some cases
	sigChan = make(chan os.Signal, 5)
	doneChan = make(chan bool, 5)

	// Handle force master mode
	if enableForceMaster {
		log.Infoln("Enabling forced master mode")
		masterCalculation.DebugAlwaysMasterOn()
	}
}

func main() {

	// start listening for shutdown interrupts
	go listenForInterrupts()

	// Create a new Kuberhealthy struct
	kuberhealthy = NewKuberhealthy()
	kuberhealthy.ListenAddr = listenAddress

	// tell Kuberhealthy to start all checks and master change monitoring
	go kuberhealthy.Start()

	// Start the web server and restart it if it crashes
	kuberhealthy.StartWebServer()
}

// listenForInterrupts watches for termination signals and acts on them
func listenForInterrupts() {
	signal.Notify(sigChan, os.Interrupt, os.Kill)
	<-sigChan
	log.Infoln("Shutting down...")
	go kuberhealthy.Shutdown()
	// wait for checks to be done shutting down before exiting
	select {
	case <-doneChan:
		log.Infoln("Shutdown gracefully completed!")
	case <-sigChan:
		log.Warningln("Shutdown forced from multiple interrupts!")
	case <-time.After(terminationGracePeriod):
		log.Errorln("Shutdown took too long.  Shutting down forcefully!")
	}
	os.Exit(0)
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
