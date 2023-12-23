package main

import (
	"strings"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"

	log "github.com/sirupsen/logrus"

	khstatev1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khstate/v1"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/health"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

// StateReflector watches the state of khstate objects and stores them in a local cache.  Then, when the current
// state of checks is requested, the CurrentStatus func can serve it rapidly from cache.  Needs to run in the
// background and can be stopped/started by simply calling `Stop()` on it.
type StateReflector struct {
	reflector        *cache.Reflector
	reflectorSigChan chan struct{} // the channel that indicates when the cache sync should stop
	resyncPeriod     time.Duration // the period for full API re-syncs
	store            cache.Store
}

// NewStateReflector creates a new StateReflector for watching the state of khstate resources on the server
func NewStateReflector(namespace string) *StateReflector {
	sr := StateReflector{}
	sr.reflectorSigChan = make(chan struct{})
	sr.resyncPeriod = time.Minute * 5

	// structure the reflector and its required elements
	khStateListWatch := cache.NewListWatchFromClient(khStateClient.RESTClient(), stateCRDResource, namespace, fields.Everything())
	sr.store = cache.NewStore(cache.MetaNamespaceKeyFunc)
	sr.reflector = cache.NewReflector(khStateListWatch, &khstatev1.KuberhealthyState{}, sr.store, sr.resyncPeriod)

	return &sr
}

// Stop halts cache sync operations.  this is async and we don't know exactly when the sync worker fully stops
func (sr *StateReflector) Stop() {
	log.Infoln("khState reflector stopping")
	if sr.reflectorSigChan != nil {
		sr.reflectorSigChan <- struct{}{}
	}
}

// Start begins the store and resync operations in the background
func (sr *StateReflector) Start() {
	log.Infoln("khState reflector starting")
	sr.reflector.Run(sr.reflectorSigChan)
}

// CurrentStatus returns the current summary of checks as known by the cache.
func (sr *StateReflector) CurrentStatus() health.State {
	log.Infoln("khState reflector fetching current status")
	state := health.NewState()

	// if the store is nil, then we just return a blank slate
	if sr.store == nil {
		log.Warningln("attempted to fetch CurrentStatus from khStateReflector, but the store was nil")
		return state
	}

	// list all objects from the storage cache
	khStateList := sr.store.List()
	for i, khStateUndefined := range khStateList {
		log.Debugln("state reflector store item from listing:", i, khStateUndefined)
		khState, ok := khStateUndefined.(*khstatev1.KuberhealthyState)
		if !ok {
			log.Warningln("attempted to convert item from state cache reflector to a khstatev1.KuberhealthyState, but the type was invalid")
			continue
		}

		log.Debugln("Getting status of check for web request to status page:", khState.GetName(), khState.GetNamespace())

		// skip the check if it has never been run before.  This prevents checks that have not yet
		// run from showing in the status page.
		if len(khState.Spec.AuthoritativePod) == 0 {
			log.Debugln("Output for", khState.GetName(), khState.GetNamespace(), "hidden from status page due to blank authoritative pod")
			continue
		}

		// parse check status from CRD and add it to the global status of errors. Skip blank errors
		for _, e := range khState.Spec.Errors {
			if len(strings.TrimSpace(e)) == 0 {
				log.Warningln("Skipped an error that was blank when adding check details to current state.")
				continue
			}
			state.AddError(e)
			log.Debugln("Status page: Setting global OK state to false due to check details not being OK")
			state.OK = false
		}

		khWorkload := determineKHWorkload(khState.Name, khState.Namespace)
		switch khWorkload {
		case khstatev1.KHCheck:
			state.CheckDetails[khState.GetNamespace()+"/"+khState.GetName()] = khState.Spec
		case khstatev1.KHJob:
			state.JobDetails[khState.GetNamespace()+"/"+khState.GetName()] = khState.Spec
		}
	}

	log.Infoln("khState reflector returning current status on", len(state.CheckDetails), "check khStates and", len(state.JobDetails), "job khStates.")
	return state
}

// determineKHWorkload uses the name and namespace of the kuberhealthy resource to determine whether its a khjob or khcheck
// This function is necessary for the CurrentStatus() function as getting the KHWorkload from the state spec returns a blank kh workload.
func determineKHWorkload(name string, namespace string) khstatev1.KHWorkload {

	var khWorkload khstatev1.KHWorkload
	log.Debugln("determineKHWorkload: determining workload:", name)

	checkPod, err := khCheckClient.KuberhealthyChecks(namespace).Get(name, v1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found") {
			log.Debugln("determineKHWorkload: Not a khcheck.")
		}
	} else {
		log.Debugln("determineKHWorkload: Found khcheck:", checkPod.Name)
		return khstatev1.KHCheck
	}

	_, err = khJobClient.KuberhealthyJobs(namespace).Get(name, v1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found") {
			log.Debugln("determineKHWorkload: Not a khjob.")
		}
	} else {
		log.Debugln("determineKHWorkload: Found khjob:", checkPod.Name)
		return khstatev1.KHJob
	}
	return khWorkload
}
