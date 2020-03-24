package main

import (
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"

	log "github.com/sirupsen/logrus"

	"github.com/Comcast/kuberhealthy/v2/pkg/health"
	"github.com/Comcast/kuberhealthy/v2/pkg/khstatecrd"
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

// NewReflector creates a new StateReflector for watching the state of khstate resoruces on the server
func NewStateReflector() *StateReflector {
	sr := StateReflector{}
	sr.reflectorSigChan = make(chan struct{})
	sr.resyncPeriod = time.Minute * 5

	// structure the reflector and its required elements
	khStateListWatch := cache.NewListWatchFromClient(khStateClient.RestClient(), stateCRDResource, "", fields.Everything())
	sr.store = cache.NewStore(cache.MetaNamespaceKeyFunc)
	sr.reflector = cache.NewReflector(khStateListWatch, &khstatecrd.KuberhealthyState{}, sr.store, sr.resyncPeriod)

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
// Returns ALL checks if the list of namespaces to look at is empty.
// Returns checks from requested namespaces if given any.
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
		khState, ok := khStateUndefined.(*khstatecrd.KuberhealthyState)
		if !ok {
			log.Warningln("attempted to convert item from state cache reflector to a khstatecrd.KuberhealthyState, but the type was invalid")
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

		// update check details struct
		state.CheckDetails[khState.GetNamespace()+"/"+khState.GetName()] = khState.Spec
	}

	log.Infoln("khState reflector returning current status on", len(state.CheckDetails), "khStates")
	return state
}
