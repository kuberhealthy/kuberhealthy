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
package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/Comcast/kuberhealthy/pkg/health"
	"github.com/Comcast/kuberhealthy/pkg/khstatecrd"
	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
	"github.com/Comcast/kuberhealthy/pkg/masterCalculation"
	"github.com/Comcast/kuberhealthy/pkg/metrics"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// Kuberhealthy represents the kuberhealhty server and its checks
type Kuberhealthy struct {
	sync.RWMutex
	Checks                []KuberhealthyCheck
	ListenAddr            string               // the listen address, such as ":80"
	checkShutdownChannels map[string]chan bool // a slice of channels used to signal shutdowns to checks
}

// NewKubeClient creates an returns a new kubernetes clientset
func (k *Kuberhealthy) NewKubeClient() (*kubernetes.Clientset, error) {
	return kubeClient.Create(kubeConfigFile)
}

// NewKuberhealthy creates a new kuberhealthy checker instance
func NewKuberhealthy() *Kuberhealthy {
	kh := &Kuberhealthy{}
	kh.checkShutdownChannels = make(map[string]chan bool)
	return kh
}

// setCheckExecutionError sets an execution error for a check name in
// its crd status
func (k *Kuberhealthy) setCheckExecutionError(checkName string, err error) {
	details := health.NewCheckDetails()
	check, err := k.getCheck(checkName)
	if err != nil {
		log.Errorln(err)
	}
	if check != nil {
		details.Namespace = check.CheckNamespace()
	}
	details.OK = false
	details.Errors = []string{"Check execution error: " + err.Error()}
	log.Debugln("Setting execution state of check", checkName, "to", details.OK, details.Errors)

	// store the check state with the CRD
	err = k.storeCheckState(checkName, details)
	if err != nil {
		log.Errorln("Was unable to write an execution error to the CRD status with error:", err)
	}
}

// AddCheck adds a check to Kuberhealthy.  Must be done before StartChecking
// is called.
func (k *Kuberhealthy) AddCheck(c KuberhealthyCheck) {
	k.Checks = append(k.Checks, c)
}

// Shutdown causes the kuberhealthy check group to shutdown gracefully
func (k *Kuberhealthy) Shutdown() {
	k.StopChecks()
	log.Debugln("All checks shutdown!")
	doneChan <- true
}

// StopChecks causes the kuberhealthy check group to shutdown gracefully.
// All checks are sent a shutdown command at the same time.
func (k *Kuberhealthy) StopChecks() {
	// send a shutdown signal to all checks
	k.sigChecks()
}

// Start inits Kuberhealthy checks and master monitoring
func (k *Kuberhealthy) Start() {

	becameMasterChan := make(chan bool)
	lostMasterChan := make(chan bool)

	// recalculate the current master on an interval
	go k.masterStatusMonitor(becameMasterChan, lostMasterChan)

	// loop and select channels to do appropriate thing when master changes
	for {
		select {
		case <-becameMasterChan:
			log.Infoln("Became master. Starting checks.")
			k.StartChecks()
		case <-lostMasterChan:
			log.Infoln("Lost master. Stopping checks.")
			k.StopChecks()
		}
	}
}

// StartChecks starts all checks concurrently and ensures they stay running
func (k *Kuberhealthy) StartChecks() {
	for _, c := range k.Checks {
		// create and log a stop signal channel here. pass into channel
		stopChan := make(chan bool, 1)
		k.addCheckStopChan(c.Name(), stopChan)

		// start the check in its own routine
		go k.runCheck(stopChan, c)
	}
}

// addCheckStopChan stores a check's shutdown channel in the checker
func (k *Kuberhealthy) addCheckStopChan(checkName string, stopChan chan bool) {
	k.Lock()
	defer k.Unlock()

	// append the check name with a unique timestamp so they dont overwrite
	now := strconv.Itoa(int(time.Now().Unix()))
	checkName = checkName + "-" + now
	k.checkShutdownChannels[checkName] = stopChan
}

// sigChecks sends signals down all check shutdown chans and removes them
// from the tracking map
func (k *Kuberhealthy) sigChecks() {
	k.Lock()
	defer k.Unlock()

	for checkName, stopChan := range k.checkShutdownChannels {
		select {
		case stopChan <- true:
			log.Debugln("Check stop channel signal sent:", checkName)
		default:
			log.Warnln("Attempted to send signal to check stop channel", checkName, "but channel did not accept send")
		}

		// remove the channel from the chceck shutdown channels listing.
		// No more shutdowns are ever sent once a check is shutdown.
		delete(k.checkShutdownChannels, checkName)
	}

}

// masterStatusMonitor calculates the master pod on a ticker.  When a
// change in master is determined that is relevant to this pod, a signal
// is sent down the appropriate became or lost channels
func (k *Kuberhealthy) masterStatusMonitor(becameMasterChan chan bool, lostMasterChan chan bool) {

	ticker := time.NewTicker(masterCalculationInterval)

	var wasMaster bool
	var isMaster bool

	for {

		client, err := kubeClient.Create(kubeConfigFile)
		if err != nil {
			log.Errorln(err)
			continue
		}

		// determine if we are currently master or not
		isMaster, err = masterCalculation.IAmMaster(client)
		if err != nil {
			log.Errorln(err)
		}

		// stop checks if we are no longer the master
		if wasMaster && !isMaster {
			log.Infoln("No longer master. Stopping all checks.")
			select {
			case lostMasterChan <- true:
			default:
			}
		}

		// start checks if we are now master
		if !wasMaster && isMaster {
			log.Infoln("I am now master. Starting checks.")
			select {
			case becameMasterChan <- true:
			default:
			}
		}

		// keep track of the previous runs master state
		wasMaster = isMaster

		<-ticker.C
	}
}

// runCheck runs a check on an interval and sets its status each run
func (k *Kuberhealthy) runCheck(stopChan chan bool, c KuberhealthyCheck) {

	// run on an interval specified by the package
	ticker := time.NewTicker(c.Interval())

	// run the check forever and write its results to the kuberhealthy
	// CRD resource for the check
	for {

		// break out if check channel is supposed to stop
		select {
		case <-stopChan:
			log.Debugln("Check", c.Name(), "stop signal received. Stopping check.")
			err := c.Shutdown()
			if err != nil {
				log.Errorln("Error stopping check", c.Name(), err)
			}
			return
		default:
		}

		log.Infoln("Running check:", c.Name())
		client, err := k.NewKubeClient()
		if err != nil {
			log.Errorln("Error creating Kubernetes client for check"+c.Name()+":", err)
			<-ticker.C
			continue
		}

		// Run the check
		err = c.Run(client)
		if err != nil {
			// set any check run errors in the CRD
			k.setCheckExecutionError(c.Name(), err)
			log.Errorln("Error running check:", c.Name(), err)
			<-ticker.C
			continue
		}
		log.Debugln("Done running check:", c.Name())

		// make a new state for this check and fill it from the check's current status
		details := health.NewCheckDetails()
		details.Namespace = c.CheckNamespace()
		details.OK, details.Errors = c.CurrentStatus()
		log.Infoln("Setting state of check", c.Name(), "to", details.OK, details.Errors)

		// store the check state with the CRD
		err = k.storeCheckState(c.Name(), details)
		if err != nil {
			log.Errorln("Error storing CRD state for check:", c.Name(), err)
		}
		<-ticker.C // wait for next run
	}
}

// storeCheckState stores the check state in its cluster CRD
func (k *Kuberhealthy) storeCheckState(checkName string, details health.CheckDetails) error {

	// make a new crd client
	client, err := khstatecrd.Client(CRDGroup, CRDVersion, kubeConfigFile)
	if err != nil {
		return err
	}

	// ensure the CRD resoruce exits
	err = ensureCRDExists(checkName, client)
	if err != nil {
		return err
	}

	// put the status on the CRD from the check
	err = setCheckCRDState(checkName, client, details)
	if err != nil {
		return err
	}

	log.Debugln("Successfully updated CRD for check:", checkName)
	return err
}

// StartWebServer starts a JSON status web server at the specified listener.
func (k *Kuberhealthy) StartWebServer() {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		err := k.prometheusMetricsHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// Assign all requests to be handled by the healthCheckHandler function
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := k.healthCheckHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	log.Infoln("Starting web services on port", k.ListenAddr)
	err := http.ListenAndServe(k.ListenAddr, nil)
	if err != nil {
		log.Errorln(err)
	}
	os.Exit(1)
}

// writeHealthCheckError writes an error to the client when things go wrong in a health check handling
func (k *Kuberhealthy) writeHealthCheckError(w http.ResponseWriter, r *http.Request, err error, state health.State) {
	// if creating a CRD client fails, then write the error back to the user
	// as well as to the error log.
	state.OK = false
	state.AddError(err.Error())
	log.Errorln(err.Error())
	// write summarized health check results back to caller
	err = state.WriteHTTPStatusResponse(w)
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
}

func (k *Kuberhealthy) prometheusMetricsHandler(w http.ResponseWriter, r *http.Request) error {
	log.Infoln("Client connected to status page from", r.RemoteAddr, r.UserAgent())
	state, err := k.getCurrentState()
	if err != nil {
		metrics.WriteMetricError(w, state)
		return err
	}
	metrics := metrics.GenerateMetrics(state)
	// write summarized health check results back to caller
	_, err = w.Write([]byte(metrics))
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}

// healthCheckHandler runs health checks against kubernetes and
// returns a status output to a web request client
func (k *Kuberhealthy) healthCheckHandler(w http.ResponseWriter, r *http.Request) error {
	log.Infoln("Client connected to status page from", r.RemoteAddr, r.UserAgent())
	state, err := k.getCurrentState()
	if err != nil {
		k.writeHealthCheckError(w, r, err, state)
		return err
	}
	// write summarized health check results back to caller
	err = state.WriteHTTPStatusResponse(w)
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}

// getCurrentState fetches the current state of all checks from their CRD objects and returns the summary as a health.State. Failures to fetch CRD state return an error.
func (k *Kuberhealthy) getCurrentState() (health.State, error) {
	// create a new set of state for this page render
	state := health.NewState()

	// create a CRD client to fetch CRD states with
	khClient, err := khstatecrd.Client(CRDGroup, CRDVersion, kubeConfigFile)
	if err != nil {
		return state, err
	}

	// fetch a client for the master calculation
	kubeClient, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		return state, err
	}

	// calculate the current master and apply it to the status output
	currentMaster, err := masterCalculation.CalculateMaster(kubeClient)
	state.CurrentMaster = currentMaster
	if err != nil {
		return state, err
	}

	// loop over every check and apply the current state to the status return
	for _, c := range k.Checks {
		log.Debugln("Getting status of check for client:", c.Name())

		// get the state from the CRD that exists for this check
		checkDetails, err := getCheckCRDState(c, khClient)
		if err != nil {
			errMessage := "System error when fetching status for check " + c.Name() + ":" + err.Error()
			log.Errorln(errMessage)
			// if there was an error getting the CRD, then use that for the check status
			// and set the check state to failed
			state.AddError(errMessage)
			log.Debugln("Status page: Setting OK to false due to an error in fetching crd state data")
			state.OK = false
			continue
		}

		// parse check status from CRD and add it to the status
		state.AddError(checkDetails.Errors...)
		if !checkDetails.OK {
			log.Debugln("Status page: Setting OK to false due to check details not being OK")
			state.OK = false
		}
		state.CheckDetails[c.Name()] = checkDetails
	}
	return state, nil
}

// getCheck returns a Kuberhealthy check object from its name, returns an error otherwise
func (k *Kuberhealthy) getCheck(name string) (KuberhealthyCheck, error) {
	for _, c := range k.Checks {
		if c.Name() == name {
			return c, nil
		}
	}
	return nil, fmt.Errorf("Could not find Kuberhealthy check with name %s", name)
}
