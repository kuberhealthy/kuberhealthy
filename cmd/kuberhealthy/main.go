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
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Comcast/kuberhealthy/pkg/checks/componentStatus"
	"github.com/Comcast/kuberhealthy/pkg/checks/daemonSet"
	"github.com/Comcast/kuberhealthy/pkg/checks/dnsStatus"
	"github.com/Comcast/kuberhealthy/pkg/checks/podRestarts"
	"github.com/Comcast/kuberhealthy/pkg/checks/podStatus"
	"github.com/Comcast/kuberhealthy/pkg/masterCalculation"
	"github.com/Comcast/kuberhealthy/pkg/metrics"
	"github.com/integrii/flaggy"
	log "github.com/sirupsen/logrus"
)

// status represents the current Kuberhealthy OK:Error state
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var listenAddress = ":8080"
var podCheckNamespaces = "kube-system"
var dnsEndpoints []string

// shutdown signal handling
var sigChan chan os.Signal
var doneChan chan bool
var terminationGracePeriodSeconds = time.Minute * 5 // keep calibrated with kubernetes terminationGracePeriodSeconds

// flags indicating that checks of specific types should be used
var enableForceMaster bool               // force master mode - for debugging
var enableDebug bool                     // enable debug logging
var DSPauseContainerImageOverride string // specify an alternate location for the DSC pause container - see #114
var DSTolerationOverride []string			 // specify an alternate list of taints to tolerate - see #178
var logLevel = "info"

var enableComponentStatusChecks = determineCheckStateFromEnvVar("COMPONENT_STATUS_CHECK")
var enableDaemonSetChecks = determineCheckStateFromEnvVar("DAEMON_SET_CHECK")
var enablePodRestartChecks = determineCheckStateFromEnvVar("POD_RESTARTS_CHECK")
var enablePodStatusChecks = determineCheckStateFromEnvVar("POD_STATUS_CHECK")
var enableDnsStatusChecks = determineCheckStateFromEnvVar("DNS_STATUS_CHECK")

// InfluxDB flags
var enableInflux = false
var influxUrl = ""
var influxUsername = ""
var influxPassword = ""
var influxDB = "http://localhost:8086"
var kuberhealthy *Kuberhealthy

// CRDGroup is a custom resource group name
const CRDGroup = "comcast.github.io"

// CRDVersion is a custom resource version
const CRDVersion = "v1"

// CRDResource is a custom resource name
const CRDResource = "khstates"

var masterCalculationInterval = time.Second * 10

func getAllLogLevel() string {
	levelStrings := []string{}
	for _, level := range log.AllLevels {
		levelStrings = append(levelStrings, level.String())
	}

	return strings.Join(levelStrings, ",")
}

func init() {

	// setup flaggy
	flaggy.SetDescription("Kuberhealthy is an in-cluster synthetic health checker for Kubernetes.")
	flaggy.String(&kubeConfigFile, "", "kubecfg", "(optional) absolute path to the kubeconfig file")
	flaggy.String(&listenAddress, "l", "listenAddress", "The port for kuberhealthy to listen on for web requests")
	flaggy.Bool(&enableComponentStatusChecks, "", "componentStatusChecks", "Set to false to disable daemonset deployment checking.")
	flaggy.Bool(&enableDaemonSetChecks, "", "daemonsetChecks", "Set to false to disable cluster daemonset deployment and termination checking.")
	flaggy.Bool(&enablePodRestartChecks, "", "podRestartChecks", "Set to false to disable pod restart checking.")
	flaggy.Bool(&enablePodStatusChecks, "", "podStatusChecks", "Set to false to disable pod lifecycle phase checking.")
	flaggy.Bool(&enableDnsStatusChecks, "", "dnsStatusChecks", "Set to false to disable DNS checks.")
	flaggy.Bool(&enableForceMaster, "", "forceMaster", "Set to true to enable local testing, forced master mode.")
	flaggy.Bool(&enableDebug, "d", "debug", "Set to true to enable debug.")
	flaggy.String(&DSPauseContainerImageOverride, "", "dsPauseContainerImageOverride", "Set an alternate image location for the pause container the daemon set checker uses for its daemon set configuration.")
	flaggy.StringSlice(&DSTolerationOverride, "", "tolerationOverride", "Specify a specific taint (in a key,value,effect format, ex. node-role.kubernetes.io/master,,NoSchedule or dedicated,someteam,NoSchedule)  to tolerate and force DaemonSetChecker to tolerate only nodes with that taint. Use the flag multiple times to add multiple tolerations. Default behavior is to tolerate all taints in the cluster.")
	flaggy.String(&podCheckNamespaces, "", "podCheckNamespaces", "The comma separated list of namespaces on which to check for pod status and restarts, if enabled.")
	flaggy.String(&logLevel, "", "log-level", fmt.Sprintf("Log level to be used one of [%s].", getAllLogLevel()))
	flaggy.StringSlice(&dnsEndpoints, "", "dnsEndpoints", "The comma separated list of dns endpoints to check, if enabled. Defaults to kubernetes.default")
	// Influx flags
	flaggy.String(&influxUsername, "", "influxUser", "Username for the InfluxDB instance")
	flaggy.String(&influxPassword, "", "influxPassword", "Password for the InfluxDB instance")
	flaggy.String(&influxUrl, "", "influxUrl", "Address for the InfluxDB instance")
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

	// handle debug logging
	if enableDebug {
		log.SetLevel(log.DebugLevel)
		masterCalculation.EnableDebug()
		log.Infoln("Enabling debug logging")
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

	go listenForInterrupts()

	// Create a new Kuberhealthy struct
	kuberhealthy = NewKuberhealthy()
	kuberhealthy.ListenAddr = listenAddress
	var metricClient metrics.Client
	if enableInflux {
		influxUrlParsed, err := url.Parse(influxUrl)
		if err != nil {
			log.Fatalln("Unable to parse influxUrl", err)
		}
		metricClient, err = metrics.NewInfluxClient(metrics.InfluxClientInput{
			Config: metrics.InfluxConfig{
				URL:      *influxUrlParsed,
				Password: influxPassword,
				Username: influxUsername,
			},
			Database: influxDB,
		})
		if err != nil {
			log.Fatalln("Unable to parse initialize connection with InfluxDB", err)
		}
	}
	kuberhealthy.MetricForwarder = metricClient

	// Split the podCheckNamespaces into a []string
	namespaces := strings.Split(podCheckNamespaces, ",")

	// Add enabled checks into Kuberhealthy

	// componentstatus checking
	if enableComponentStatusChecks {
		kuberhealthy.AddCheck(componentStatus.New())
	}

	// daemonset checking
	if enableDaemonSetChecks {
		dsc, err := daemonSet.New()
		if err != nil {
			log.Fatalln("unable to create daemonset checker:", err)
		}
		// allow the user to override the image used by the DSC - see #114
		if len(DSPauseContainerImageOverride) > 0 {
			log.Info("Setting DS pause container override image to:", DSPauseContainerImageOverride)
			dsc.PauseContainerImage = DSPauseContainerImageOverride
		}
		// allow the user to override the tolerations used by the DSC - see #178
		if len(DSPauseContainerImageOverride) > 0 {
			log.Info("Setting DS Tolerations to:", DSTolerationOverride)
			DSTolerationOverrideList, err := dsc.ParseTolerationOverride(DSTolerationOverride)
			if err != nil {
				log.Errorln("Error parsing tolerationOverride", err)
			}
			dsc.Tolerations = DSTolerationOverrideList
			}
		kuberhealthy.AddCheck(dsc)
	}

	// pod restart checking
	if enablePodRestartChecks {
		for _, namespace := range namespaces {
			n := strings.TrimSpace(namespace)
			if len(n) > 0 {
				kuberhealthy.AddCheck(podRestarts.New(n))
			}
		}
	}

	// pod status checking
	if enablePodStatusChecks {
		for _, namespace := range namespaces {
			n := strings.TrimSpace(namespace)
			if len(n) > 0 {
				kuberhealthy.AddCheck(podStatus.New(n))
			}
		}
	}

	// dns resolution checking
	if enableDnsStatusChecks {
		kuberhealthy.AddCheck(dnsStatus.New(dnsEndpoints))
	}

	// Tell Kuberhealthy to start all checks and master change monitoring
	go kuberhealthy.Start()

	// Start the web server and restart it if it crashes
	kuberhealthy.StartWebServer()

}

// listenForInterrupts watches for termination singnals and acts on them
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
	case <-time.After(terminationGracePeriodSeconds):
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
