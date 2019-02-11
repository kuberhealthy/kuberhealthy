/* Copyright 2018 Comcast Cable Communications Management, LLC
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at
       http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Kuberhealthy is an enhanced health check for Kubernetes clusters.
package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/Comcast/kuberhealthy/pkg/checks/componentStatus"
	"github.com/Comcast/kuberhealthy/pkg/checks/daemonSet"
	"github.com/Comcast/kuberhealthy/pkg/checks/podRestarts"
	"github.com/Comcast/kuberhealthy/pkg/checks/podStatus"
	"github.com/Comcast/kuberhealthy/pkg/masterCalculation"
	"github.com/integrii/flaggy"
	log "github.com/sirupsen/logrus"
)

// status represents the current Kuberhealthy OK:Error state
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var listenAddress = ":8080"
var podCheckNamespaces = "kube-system"

// shutdown signal handling
var sigChan chan os.Signal
var doneChan chan bool
var terminationGracePeriodSeconds = time.Minute * 5 // keep calibrated with kubernetes terminationGracePeriodSeconds

// flags indicating that checks of specific types should be used
var enableComponentStatusChecks = true // do componentstatus checking
var enableDaemonSetChecks = true       // do daemon set restart checking
var enablePodRestartChecks = true      // do pod restart checking
var enablePodStatusChecks = true       // do pod status checking
var enableForceMaster bool             // force master mode - for debugging
var enableDebug bool                   // enable deubug logging

var kuberhealthy *Kuberhealthy

// values for CRD interaction
const CRDGroup = "comcast.github.io"
const CRDVersion = "v1"
const CRDResource = "khstates"

var masterCalculationInterval = time.Second * 10

func init() {
	flaggy.SetDescription("Kuberhealthy is an in-cluster synthetic health checker for Kubernetes.")
	flaggy.String(&kubeConfigFile, "", "kubecfg", "(optional) absolute path to the kubeconfig file")
	flaggy.String(&listenAddress, "l", "listenAddress", "The port for kuberhealthy to listen on for web requests")
	flaggy.Bool(&enableComponentStatusChecks, "", "componentStatusChecks", "Set to false to disable daemonset deployment checking.")
	flaggy.Bool(&enableDaemonSetChecks, "", "daemonsetChecks", "Set to false to disable cluster daemonset deployment and termination checking.")
	flaggy.Bool(&enablePodRestartChecks, "", "podRestartChecks", "Set to false to disable pod restart checking.")
	flaggy.Bool(&enablePodStatusChecks, "", "podStatusChecks", "Set to false to disable pod lifecycle phase checking.")
	flaggy.Bool(&enableForceMaster, "", "forceMaster", "Set to true to enable local testing, forced master mode.")
	flaggy.Bool(&enableDebug, "d", "debug", "Set to true to enable debug.")
	flaggy.String(&podCheckNamespaces, "", "podCheckNamespaces", "The comma separated list of namespaces on which to check for pod status and restarts, if enabled.")
	flaggy.Parse()

	// log to stdout and set the level to info by default
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
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
