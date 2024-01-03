package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	khcheckv1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khcheck/v1"
	khjobv1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khjob/v1"
	khstatev1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khstate/v1"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/status"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/health"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/masterCalculation"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/metrics"
)

// Kuberhealthy represents the kuberhealthy server and its checks
type Kuberhealthy struct {
	Checks             []*external.Checker
	ListenAddr         string // the listen address, such as ":80"
	MetricForwarder    metrics.Client
	overrideKubeClient *kubernetes.Clientset
	cancelChecksFunc   context.CancelFunc // invalidates the context of all running checks
	cancelReaperFunc   context.CancelFunc // invalidates the context of the reaper
	wg                 sync.WaitGroup     // used to track running checks
	shutdownCtxFunc    context.CancelFunc // used to shutdown the main control select
	stateReflector     *StateReflector    // a reflector that can cache the current state of the khState resources
	TargetNamespace    string             // the namespace that this instance will operate on. to include all namespaces, set this to a blank
	config             *Config            // the config struct loaded at setup
}

// NewKuberhealthy creates a new kuberhealthy checker instance restricted to the desired
// namespace.  If this instance should apply to all namespaces, pass a blank here.
func NewKuberhealthy(cfg *Config) *Kuberhealthy {
	kh := &Kuberhealthy{
		TargetNamespace: cfg.TargetNamespace,
		ListenAddr:      cfg.ListenAddress,
		config:          cfg,
	}
	kh.stateReflector = NewStateReflector(kh.TargetNamespace)
	return kh
}

// setCheckExecutionError sets an execution error for a check name in
// its crd status
func (k *Kuberhealthy) setCheckExecutionError(checkName string, checkNamespace string, exErr error) error {
	details := khstatev1.NewWorkloadDetails(khstatev1.KHCheck)
	check, err := k.getCheck(checkName, checkNamespace)
	if err != nil {
		return err
	}
	if check.Namespace != "" {
		details.Namespace = check.CheckNamespace()
	}
	details.OK = false
	details.Errors = []string{"Check execution error: " + exErr.Error()}

	// we need to maintain the current UUID, which means fetching it first
	khc, err := k.getCheck(checkName, checkNamespace)
	if err != nil {
		return fmt.Errorf("error when setting execution error on check %s %s %w", checkName, checkNamespace, err)
	}

	checkState, err := getCheckState(khc)
	if err != nil {
		return fmt.Errorf("error when setting execution error on check (getting check state for current UUID) %s %s %w", checkName, checkNamespace, err)
	}
	details.CurrentUUID = checkState.CurrentUUID
	log.Debugln("Setting execution state of check", checkName, "to", details.OK, details.Errors, details.CurrentUUID, details.GetKHWorkload())

	// store the check state with the CRD
	err = k.storeCheckState(checkName, checkNamespace, details)
	if err != nil {
		return fmt.Errorf("unable to write an execution error to the CRD status with error: %w", err)
	}
	return nil
}

// setJobExecutionError sets an execution error for a job name in its crd status
func (k *Kuberhealthy) setJobExecutionError(jobName string, jobNamespace string, exErr error) error {
	details := khstatev1.NewWorkloadDetails(khstatev1.KHJob)
	job, err := k.getJob(jobName, jobNamespace)
	if err != nil {
		return err
	}
	if job.Namespace != "" {
		details.Namespace = job.CheckNamespace()
	}
	details.OK = false
	details.Errors = []string{"Job execution error: " + exErr.Error()}

	// we need to maintain the current UUID, which means fetching it first
	khj, err := k.getJob(jobName, jobNamespace)
	if err != nil {
		return fmt.Errorf("error when setting execution error on job %s %s %w", jobName, jobNamespace, err)
	}
	jobState, err := getJobState(khj)
	if err != nil {
		return fmt.Errorf("error when setting execution error on job (getting job state for current UUID) %s %s %w", jobName, jobNamespace, err)
	}
	details.CurrentUUID = jobState.CurrentUUID

	log.Debugln("Setting execution state of job", jobName, "to", details.OK, details.Errors, details.CurrentUUID, details.GetKHWorkload())

	// store the check state with the CRD
	err = k.storeCheckState(jobName, jobNamespace, details)
	if err != nil {
		return fmt.Errorf("unable to write an execution error to the CRD status with error: %w", err)
	}
	return nil
}

// AddCheck adds a check to Kuberhealthy.  Must be done before Start or StartChecks
// are called.
func (k *Kuberhealthy) AddCheck(c *external.Checker) {
	k.Checks = append(k.Checks, c)
}

// Shutdown causes the kuberhealthy chec k group to shutdown gracefully
func (k *Kuberhealthy) Shutdown(doneChan chan struct{}) {
	if k.shutdownCtxFunc != nil {
		log.Infoln("shutdown: aborting control context")
		k.shutdownCtxFunc() // stop the control system
	}
	time.Sleep(5 * time.Second) // help prevent more checks from starting in a race before control system stop happens
	log.Infoln("shutdown: stopping checks")
	k.StopChecks() // stop all checks
	log.Infoln("shutdown: ready for main program shutdown")
	doneChan <- struct{}{}
}

// StopChecks causes the kuberhealthy check group to shutdown gracefully.
// All checks are sent a shutdown command at the same time.
func (k *Kuberhealthy) StopChecks() {

	log.Infoln("control:", len(k.Checks), "checks stopping...")
	if k.cancelChecksFunc != nil {
		k.cancelChecksFunc()
	}

	// call a shutdown on all checks concurrently
	for _, c := range k.Checks {
		go func(c *external.Checker, kh *Kuberhealthy) {
			log.Infoln("control: check", c.Name(), "stopping...")
			err := c.Shutdown()
			if err != nil {
				log.Errorln("control: ERROR stopping check", c.Name(), err)
			}
			kh.wg.Done()
			log.Infoln("control: check", c.Name(), "stopped")
		}(c, k)
	}

	// wait for all checks to stop cleanly
	log.Infoln("control: waiting for all checks to stop")
	k.wg.Wait()

	log.Infoln("control: all checks stopped.")
}

// Start inits Kuberhealthy checks and master monitoring
func (k *Kuberhealthy) Start(ctx context.Context) {

	// start the khState reflector
	go k.stateReflector.Start()

	// if influxdb is enabled, configure it
	if cfg.EnableInflux {
		k.configureInfluxForwarding()
	}

	// Start the web server and restart it if it crashes
	go k.StartWebServer()

	// find all the external checks from the khcheckcrd resources on the cluster and keep them in sync.
	// use rate limiting to avoid reconfiguration spam
	maxUpdateInterval := time.Second * 10
	externalChecksUpdateChan := make(chan struct{}, 50)
	externalChecksUpdateChanLimited := make(chan struct{}, 50)
	go notifyChanLimiter(maxUpdateInterval, externalChecksUpdateChan, externalChecksUpdateChanLimited)
	go k.monitorExternalChecks(ctx, externalChecksUpdateChan)

	// we use two channels to indicate when we gain or lose master status. use rate limiting to avoid
	// reconfiguration spam
	becameMasterChan := make(chan struct{}, 10)
	lostMasterChan := make(chan struct{}, 10)
	go k.masterMonitor(ctx, becameMasterChan, lostMasterChan)

	// monitor for kuberhealthy jobs and trigger when a new job is added
	go k.monitorKHJobs(ctx)

	// get notified when kuberhealthy configuration is reloaded
	configReloadChan := make(chan struct{})
	go configReloadNotifier(ctx, configReloadChan)

	// loop and select channels to do appropriate thing when:
	// - master kuberhealthy pod changes
	// - new khchecks are added or modified
	// - kuberhealthy configuration changes
	for {
		select {
		case <-ctx.Done(): // we are shutting down
			log.Infoln("control: shutting down from context abort...")
			return
		case <-becameMasterChan: // we have become the current master instance and should run checks
			// reset checks and re-add from configuration settings
			log.Infoln("control: Became master. Reconfiguring and starting checks.")
			k.StartChecks(ctx)
			k.StartReaper(ctx)
		case <-lostMasterChan: // we are no longer master
			log.Infoln("control: Lost master. Stopping checks.")
			k.StopChecks()
			k.StopReaper()
		case <-externalChecksUpdateChanLimited: // external check change detected
			log.Infoln("control: Witnessed a khcheck resource change...")

			// if we are master, stop, reconfigure our khchecks, and start again with the new configuration
			if isMaster {
				log.Infoln("control: Reloading external check configurations due to khcheck update")
				k.RestartChecks(ctx)
				k.RestartReaper(ctx)
			}
		case <-configReloadChan:
			log.Infoln("control: Witnessed a kuberhealthy configuration change...")

			// if we are master, stop, reconfigure our khchecks, and start again with the new configuration
			if isMaster {
				log.Infoln("control: Reloading external check configurations due to kuberhealthy configuration update")
				k.RestartChecks(ctx)
				k.RestartReaper(ctx)
			}
		}
	}
}

// StartReaper starts the check reaper
func (k *Kuberhealthy) StartReaper(ctx context.Context) {
	reaperCtx, reaperCtxCancel := context.WithCancel(ctx)
	k.cancelReaperFunc = reaperCtxCancel
	go reaper(reaperCtx, k.TargetNamespace)
}

// StopReaper stops the check reaper
func (k *Kuberhealthy) StopReaper() {
	if k.cancelReaperFunc != nil {
		k.cancelReaperFunc()
	}
}

// RestartReaper resrtarts the check reaper
func (k *Kuberhealthy) RestartReaper(ctx context.Context) {
	k.StopReaper()
	k.StartReaper(ctx)
}

// RestartChecks does a stop and start on all kuberhealthy checks
func (k *Kuberhealthy) RestartChecks(ctx context.Context) {
	k.StopChecks()
	k.StartChecks(ctx)
}

// khStateResourceReaper runs reapKHStateResources on an interval until the context for it is canceled
func (k *Kuberhealthy) khStateResourceReaper(ctx context.Context, namespace string) {

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	log.Infoln("khState reaper: starting up")

	for {
		select {
		case <-ticker.C:
			log.Infoln("khState reaper: starting to run an audit")
			err := k.reapKHStateResources(ctx, namespace)
			if err != nil {
				log.Errorln("khState reaper: Error when reaping khState resources:", err)
			}
		case <-ctx.Done():
			log.Infoln("khState reaper: stopping")
			return
		}
	}

}

// reapKHStateResources runs a single audit on khState resources.  Any that don't have a matching khCheck are
// deleted.
func (k *Kuberhealthy) reapKHStateResources(ctx context.Context, namespace string) error {

	// list all khStates in the cluster
	khStates, err := khStateClient.KuberhealthyStates(namespace).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("khState reaper: error listing khStates for reaping: %w", err)
	}

	khChecks, err := k.listKHChecks(k.TargetNamespace)
	if err != nil {
		return fmt.Errorf("khState reaper: error listing unstructured khChecks: %w", err)
	}

	khJobs, err := khJobClient.KuberhealthyJobs(namespace).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("khState reaper: error listing khJobs for reaping: %w", err)
	}

	log.Infoln("khState reaper: analyzing", len(khStates.Items), "khState resources")

	// any khState that does not have a matching khCheck should be deleted (ignore errors)
	for _, khState := range khStates.Items {
		log.Debugln("khState reaper: analyzing khState", khState.GetName(), "in", khState.GetName())
		var foundKHCheck bool
		for _, kc := range khChecks.Items {
			if err != nil {
				log.Errorln("Error converting unstructured object to khcheck:", err)
				continue
			}
			log.Debugln("khState reaper:", kc.GetName(), "==", khState.GetName(), "&&", kc.GetNamespace(), "==", khState.GetNamespace())
			if kc.GetName() == khState.GetName() && kc.GetNamespace() == khState.GetNamespace() {
				log.Infoln("khState reaper:", khState.GetName(), "in", khState.GetNamespace(), "is still valid")
				foundKHCheck = true
				break
			}
		}

		var foundKHJob bool
		for _, kj := range khJobs.Items {
			log.Debugln("khState reaper:", kj.GetName(), "==", khState.GetName(), "&&", kj.GetNamespace(), "==", khState.GetNamespace())
			if kj.GetName() == khState.GetName() && kj.GetNamespace() == khState.GetNamespace() {
				log.Infoln("khState reaper:", khState.GetName(), "in", khState.GetNamespace(), "is still valid")
				foundKHJob = true
				break
			}
		}

		// if we didn't find a matching khCheck or khJob, delete the rogue khState
		if !foundKHCheck && !foundKHJob {
			log.Infoln("khState reaper: removing khState", khState.GetName(), "in", khState.GetNamespace())
			err := khStateClient.KuberhealthyStates(khState.GetNamespace()).Delete(khState.GetName(), &metav1.DeleteOptions{})
			if err != nil {
				log.Errorln(fmt.Errorf("khState reaper: error when removing invalid khstate: %w", err))
			}
		}
	}

	return nil

}

// monitorKHJobs watches for newly added KHJobs and triggers them
func (k *Kuberhealthy) monitorKHJobs(ctx context.Context) {

	log.Debugln("Spawned watcher for KH jobs")

	for {
		log.Debugln("Starting a watch for khcheck jobs")

		// wait a second so we don't retry too quickly on error
		time.Sleep(time.Second)

		watcher, err := khJobClient.KuberhealthyJobs(k.TargetNamespace).Watch(metav1.ListOptions{})
		if err != nil {
			log.Errorln("error watching for khjob objects:", err)
			continue
		}

		// watch for the watcher context to end, or the parent context.  If the parent context ends, we close the watcher.
		// if the watcher context ends, we shut down this go routine to prevent a leak as it restarts
		watcherCtx, watcherCtxCancel := context.WithCancel(context.Background())
		go func(watchCtx context.Context, ctx context.Context, watcher watch.Interface) {
			select {
			case <-watchCtx.Done():
				break
			case <-ctx.Done():
				watcher.Stop()
			}
			log.Debugln("khjob monitor watch stopping")
		}(watcherCtx, ctx, watcher)

		for khj := range watcher.ResultChan() {
			switch khj.Type {
			// Watch only for added events since we only care about khjobs that added / created.
			// Ignore all other event types.
			case watch.Added:
				log.Debugln("khjob monitor saw an added event")
				kj := khj.Object.(*khjobv1.KuberhealthyJob)
				if verifyNewKHJob(kj.Name, kj.Namespace) {
					log.Infoln("khJob is newly added, triggering khjob:", kj.Name)
					k.triggerKHJob(ctx, *kj)
					continue
				}
				log.Debugln("KHJob is not new, in phase:", kj.Spec.Phase, "Skipping added event")
				continue
			case watch.Error:
				log.Debugln("khjob monitor saw an error event")
				e := khj.Object.(*metav1.Status)
				log.Errorln("Error when watching khjobs:", e.Reason)
				continue
			default:
				log.Warningln("khjob monitor saw an unknown event type and ignored it:", khj.Type)
				continue
			}
		}

		// if the watcher breaks, shutdown the parent context monitor go routine
		watcherCtxCancel()

		select {
		case <-ctx.Done():
			log.Debugln("khjob monitor closing due to context cancellation")
			return
		default:
		}
	}
}

// listKHChecks lists all kuberhealthy checks in the specified namespace
func (k *Kuberhealthy) listKHChecks(namespace string) (khcheckv1.KuberhealthyCheckList, error) {
	return khCheckClient.KuberhealthyChecks(namespace).List(metav1.ListOptions{})
}

// getKHCheck gets the specified khcheck in the specified namespace
func (k *Kuberhealthy) getKHCheck(namespace string, checkName string) (khcheckv1.KuberhealthyCheck, error) {
	return khCheckClient.KuberhealthyChecks(namespace).Get(checkName, metav1.GetOptions{})
}

// listKHStates lists all kuberhealthy states in the specified namespace
func (k *Kuberhealthy) listKHStates(namespace string) (khstatev1.KuberhealthyStateList, error) {
	return khStateClient.KuberhealthyStates(namespace).List(metav1.ListOptions{})
}

// getKHState gets the specified khstate in the specified namespace
func (k *Kuberhealthy) getKHState(namespace string, checkName string) (khstatev1.KuberhealthyState, error) {
	return khStateClient.KuberhealthyStates(namespace).Get(checkName, metav1.GetOptions{})
}

// watchForKHCheckChanges watches for changes to khcheck objects and returns them through the specified channel
func (k *Kuberhealthy) watchForKHCheckChanges(ctx context.Context, c chan struct{}) {

	log.Debugln("Spawned watcher for KH check changes")

	for {
		log.Debugln("Starting a watch for khcheck object changes")

		// wait a second so we don't retry too quickly on error
		time.Sleep(time.Second)

		// start a watch on khcheck resources
		watcher, err := khCheckClient.KuberhealthyChecks(k.TargetNamespace).Watch(metav1.ListOptions{})
		// watcher, err := watchUnstructuredKHChecks(ctx, k.TargetNamespace)
		if err != nil {
			log.Errorln("error creating watcher for khcheck objects:", err)
			continue
		}

		// watch for the watcher context to end, or the parent context.  If the parent context ends, we close the watcher.
		// if the watcher context ends, we shut down this go routine to prevent a leak as it restarts
		watcherCtx, watcherCtxCancel := context.WithCancel(context.Background())
		go func(watchCtx context.Context, ctx context.Context, watcher watch.Interface) {
			select {
			case <-watchCtx.Done():
				break
			case <-ctx.Done():
				watcher.Stop()
			}
			log.Debugln("khcheck change monitor watch stopping")
		}(watcherCtx, ctx, watcher)

		// loop over results and return them to the calling channel until we hit an error, then close and restart
		for khc := range watcher.ResultChan() {
			switch khc.Type {
			case watch.Added:
				log.Debugln("khcheck monitor saw an added event")
				c <- struct{}{}
			case watch.Modified:
				log.Debugln("khcheck monitor saw a modified event")
				c <- struct{}{}
			case watch.Deleted:
				log.Debugln("khcheck monitor saw a deleted event")
				c <- struct{}{}
			case watch.Error:
				log.Debugln("khcheck monitor saw an error event")
				e := khc.Object.(*metav1.Status)
				log.Errorln("Error when watching for khcheck changes:", e.Reason)
				continue
			default:
				log.Warningln("khcheck monitor saw an unknown event type and ignored it:", khc.Type)
			}
		}

		// if the watcher breaks, shutdown the parent context monitor go routine
		watcherCtxCancel()

		select {
		case <-ctx.Done():
			log.Debugln("khcheck monitor closing due to context cancellation")
			return
		default:
		}
	}
}

func verifyNewKHJob(khJobName string, khJobNamespace string) bool {

	kj, err := khJobClient.KuberhealthyJobs(khJobNamespace).Get(khJobName, metav1.GetOptions{})
	if err != nil {
		log.Debugln(khJobName, "Error getting khjob:", khJobName, err)
		return false
	}
	log.Debugln("Found khjob:", kj.Name, "in job phase:", kj.Spec.Phase)

	return kj.Spec.Phase == ""
}

// monitorExternalChecks watches for changes to the external check CRDs
func (k *Kuberhealthy) monitorExternalChecks(ctx context.Context, notify chan struct{}) {

	// make a map of resource versions so we know when things change
	knownSettings := make(map[string]khcheckv1.CheckConfig)

	// start watching for events to changes in the background
	c := make(chan struct{})
	go k.watchForKHCheckChanges(ctx, c)

	// each time  we see a change in our khcheck structs, we should look at every object to see if something has changed
	for {

		// wait for the change channel to detect a change before scanning again
		<-c
		log.Debugln("Change notification received. Scanning for external check changes...")

		khChecks, err := k.listKHChecks(k.TargetNamespace)
		if err != nil {
			log.Errorln("error listing unstructured khChecks: %w", err)
			continue
		}

		// this bool indicates if we should send a change signal to the channel
		var foundChange bool

		// if a khcheck has been deleted, then we signal for change and purge it from the knownSettings map.
		for mapName := range knownSettings {
			var existsInItems bool // indicates the item exists in the item listing

			for _, kc := range khChecks.Items {

				itemMapName := kc.Namespace + "/" + kc.Name
				if itemMapName == mapName {
					existsInItems = true
					break
				}
			}
			if !existsInItems {
				log.Debugln("Detected khcheck deletion for", mapName)
				delete(knownSettings, mapName)
				foundChange = true
			}
		}

		for _, kc := range khChecks.Items {
			if err != nil {
				log.Errorln("Error converting unstructured object to khcheck:", err)
				continue
			}

			mapName := kc.Namespace + "/" + kc.Name

			log.Debugln("Scanning khcheck CRD", mapName, "for changes since last seen...")

			if len(kc.Namespace) < 1 {
				log.Warning("Got khcheck update from object with no namespace...")
				continue
			}
			if len(kc.Name) < 1 {
				log.Warning("Got khcheck update from object with no name...")
				continue
			}

			// if we don't know about this check yet, just store the state and continue.  The check is already
			// loaded on the first check configuration run.
			_, exists := knownSettings[mapName]
			if !exists {
				log.Debugln("First time seeing khcheck of name", mapName)
				knownSettings[mapName] = kc.Spec
				foundChange = true
			}

			// check if run interval has changed
			if knownSettings[mapName].RunInterval != kc.Spec.RunInterval {
				log.Debugln("The khcheck run interval for", mapName, "has changed.")
				foundChange = true
			}

			// check if run timeout has changed
			if knownSettings[mapName].Timeout != kc.Spec.Timeout {
				log.Debugln("The khcheck timeout for", mapName, "has changed.")
				foundChange = true
			}

			// check if extraLabels has changed
			if !foundChange && !reflect.DeepEqual(knownSettings[mapName].ExtraLabels, kc.Spec.ExtraLabels) {
				log.Debugln("The khcheck extra labels for", mapName, "has changed.")
				foundChange = true
			}

			// check if extraAnnotations has changed
			if !foundChange && !reflect.DeepEqual(knownSettings[mapName].ExtraAnnotations, kc.Spec.ExtraAnnotations) {
				log.Debugln("The khcheck extra annotations for", mapName, "has changed.")
				foundChange = true
			}

			// check if CheckConfig has changed (PodSpec)
			if !foundChange && !reflect.DeepEqual(knownSettings[mapName].PodSpec, kc.Spec.PodSpec) {
				log.Debugln("The khcheck for", mapName, "has changed.")
				foundChange = true
			}

			// finally, update known settings before continuing to the next interval
			knownSettings[mapName] = kc.Spec
		}

		// if a change was detected, we signal the notify channel
		if foundChange {
			log.Debugln("Signaling that a change was found in external check configuration")
			notify <- struct{}{}
		}
	}
}

// setExternalChecks syncs up the state of the external-checks installed in this
// Kuberhealthy struct.
func (k *Kuberhealthy) addExternalChecks(ctx context.Context) error {

	log.Debugln("Fetching khcheck configurations...")

	khChecks, err := k.listKHChecks(k.TargetNamespace)
	if err != nil {
		return err
	}

	log.Debugln("Found", len(khChecks.Items), "external checks to load")

	// iterate on each check CRD resource and add it as a check
	for _, kc := range khChecks.Items {
		if err != nil {
			log.Errorln("Error converting unstructured object to khcheck:", err)
			continue
		}
		log.Debugln("Loading check CRD:", kc.Name)

		log.Debugf("External check custom resource loaded: %v", kc)

		// create a new kubernetes client for this external checker
		log.Infoln("Enabling external check:", kc.Name)
		c := external.New(kubernetesClient, &kc, khCheckClient, khStateClient, cfg.ExternalCheckReportingURL)

		// parse the run interval string from the custom resource and setup the run interval
		c.RunInterval, err = time.ParseDuration(kc.Spec.RunInterval)
		if err != nil {
			log.Errorln("Error parsing duration for check", c.CheckName, "in namespace", c.Namespace, err)
			log.Errorln("Defaulting check to a runtime of ten minutes.")
			c.RunInterval = DefaultRunInterval
		}

		log.Debugln("RunInterval for check:", c.CheckName, "set to", c.RunInterval)

		// parse the user specified timeout if present
		c.RunTimeout = DefaultTimeout
		if len(kc.Spec.Timeout) > 0 {
			c.RunTimeout, err = time.ParseDuration(kc.Spec.Timeout)
			if err != nil {
				log.Errorln("Error parsing timeout for check", c.CheckName, "in namespace", c.Namespace, err)
				log.Errorln("Defaulting check to a timeout of", DefaultTimeout)
			}
		}

		log.Debugln("RunTimeout for check:", c.CheckName, "set to", c.RunTimeout)

		// add on extra annotations and labels
		if c.ExtraAnnotations != nil {
			log.Debugln("External check setting extra annotations:", c.ExtraAnnotations)
			c.ExtraAnnotations = kc.Spec.ExtraAnnotations
		}
		if c.ExtraLabels != nil {
			log.Debugln("External check setting extra labels:", c.ExtraLabels)
			c.ExtraLabels = kc.Spec.ExtraLabels
		}
		log.Debugln("External check labels and annotations:", c.ExtraLabels, c.ExtraAnnotations)

		// add the check into the checker
		k.AddCheck(c)
	}

	return nil
}

// addExternalJobs syncs up the state of the all jobs installed in this Kuberhealthy struct.
func (k *Kuberhealthy) configureJob(job khjobv1.KuberhealthyJob) *external.Checker {

	log.Debugln("Loading job CRD:", job.Name)

	// create a new kubernetes client for this external checker
	log.Infoln("Enabling external job:", job.Name)
	kj := external.NewJob(kubernetesClient, &job, khJobClient, khStateClient, cfg.ExternalCheckReportingURL)

	var err error
	// parse the user specified timeout if present
	kj.RunTimeout = DefaultTimeout
	if len(job.Spec.Timeout) > 0 {
		kj.RunTimeout, err = time.ParseDuration(job.Spec.Timeout)
		if err != nil {
			log.Errorln("Error parsing timeout for check", kj.CheckName, "in namespace", kj.Namespace, err)
			log.Errorln("Defaulting check to a timeout of", DefaultTimeout)
		}
	}

	log.Debugln("RunTimeout for job:", kj.CheckName, "set to", kj.RunTimeout)

	// add on extra annotations and labels
	if kj.ExtraAnnotations != nil {
		log.Debugln("External job setting extra annotations:", kj.ExtraAnnotations)
		kj.ExtraAnnotations = job.Spec.ExtraAnnotations
	}
	if kj.ExtraLabels != nil {
		log.Debugln("External job setting extra labels:", kj.ExtraLabels)
		kj.ExtraLabels = job.Spec.ExtraLabels
	}
	log.Debugln("External job labels and annotations:", kj.ExtraLabels, kj.ExtraAnnotations)
	return kj
}

// triggerKHJob checks if its master, sets the context, and runs the khjob in a goroutine
func (k *Kuberhealthy) triggerKHJob(ctx context.Context, job khjobv1.KuberhealthyJob) {

	log.Debugln("khjob trigger, isMaster:", isMaster)
	// only the master pod should be running khjobs or khjobs are duplicated
	if isMaster {
		go k.runJob(ctx, job)
	}
}

// StartChecks starts all checks concurrently and ensures they stay running
func (k *Kuberhealthy) StartChecks(ctx context.Context) {
	// wait for all check wg to be done, just in case
	k.wg.Wait()

	log.Infoln("control: Reloading check configuration...")
	k.configureChecks(ctx)

	// sleep to make a more graceful switch-up during lots of master and check changes coming in
	log.Infoln("control:", len(k.Checks), "checks starting!")

	// create a context for checks to abort with
	checkGroupCtx, cancelFunc := context.WithCancel(ctx)
	k.cancelChecksFunc = cancelFunc

	// start each check with this check group's context
	for _, c := range k.Checks {
		k.wg.Add(1)
		// start the check in its own routine
		go k.runCheck(checkGroupCtx, c)
	}

	// spin up the khState reaper with a context after checks have been configured and started
	log.Infoln("control: reaper starting!")
	go k.khStateResourceReaper(ctx, k.TargetNamespace)
}

// masterStatusWatcher watches for master change events and updates the global upcomingMasterState along
// with the global lastMasterChangeTime
func (k *Kuberhealthy) masterStatusWatcher(ctx context.Context) {

	// continue reconnecting to the api to resume the pod watch if it ends
	for {
		log.Debugln("master status watcher starting up...")

		// don't retry our watch too fast
		time.Sleep(time.Second * 5)

		// setup a pod watching client for kuberhealthy pods
		watcher, err := kubernetesClient.CoreV1().Pods(podNamespace).Watch(ctx, metav1.ListOptions{
			LabelSelector: "app=kuberhealthy",
		})
		if err != nil {
			log.Errorln("error when attempting to watch for kuberhealthy pod changes:", err)
			continue
		}

		// watch for the parent context to expire as well as this watch context. if the parent context expires,
		// then we stop the watcher.  if the watcher context expires, we terminate the go routine to prevent a
		// goroutine leak
		watcherCtx, watcherCtxCancel := context.WithCancel(context.Background())
		go func(watchCtx context.Context, ctx context.Context, watcher watch.Interface) {
			select {
			case <-watchCtx.Done():
				break
			case <-ctx.Done():
				watcher.Stop()
			}
			log.Debugln("master status monitor watch stopping")
		}(watcherCtx, ctx, watcher)

		// on each update from the watch, we re-check our master status.
		for range watcher.ResultChan() {

			// update the time we last saw a master event
			lastMasterChangeTime = time.Now()

			// determine if we are becoming master or not
			var err error
			upcomingMasterState, err = masterCalculation.IAmMaster(kubernetesClient)
			if err != nil {
				log.Errorln(err)
			}

			// update the time we last saw a master event
			log.Debugln("master status monitor saw a master event")
			lastMasterChangeTime = time.Now()
		}

		// cancel the watcher by revoking its context
		watcherCtxCancel()

		// if the context has expired, then shut down the master status watcher entirely
		select {
		case <-ctx.Done():
			log.Debugln("master status monitor stopping due to context cancellation")
			return
		default:
		}
	}
}

// masterMonitor periodically evaluates the current and upcoming master state
// and makes it so when appropriate
func (k *Kuberhealthy) masterMonitor(ctx context.Context, becameMasterChan chan struct{}, lostMasterChan chan struct{}) {

	// watch master pod event changes and recalculate the current master state of this pdo with each
	go k.masterStatusWatcher(ctx)

	interval := time.Second * 10

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// on each tick, we ensure that enough time has passed since the last master change
	// event, then we calculate if we should become or lose master.
	for range ticker.C {

		if time.Since(lastMasterChangeTime) < interval {
			log.Println("control: waiting for master changes to settle...")
			continue
		}

		// dupe the global to prevent races
		goingToBeMaster := upcomingMasterState

		// stop checks if we are no longer the master
		if goingToBeMaster && !isMaster {
			becameMasterChan <- struct{}{}
		}

		// start checks if we are now master
		if !goingToBeMaster && isMaster {
			lostMasterChan <- struct{}{}
		}

		// refresh global isMaster state
		isMaster = goingToBeMaster
	}
}

// runJob runs the job and sets its status
func (k *Kuberhealthy) runJob(ctx context.Context, job khjobv1.KuberhealthyJob) {

	log.Infoln("control: Loading job configuration...")
	j := k.configureJob(job)

	log.Println("Starting kuberhealthy job:", j.CheckNamespace(), "/", j.Name())
	// break out if context cancels
	select {
	case <-ctx.Done():
		// we don't need to call a job shutdown here because the same func that cancels this context calls
		// shutdown on all the jobs configured in the kuberhealthy struct.
		log.Infoln("Shutting down job run due to context cancellation:", j.Name(), "in namespace", j.CheckNamespace())
		return
	default:
	}

	// Run the job
	log.Infoln("Running job:", j.Name())
	// Record job run start time
	jobStartTime := time.Now()
	// set KHJob phase to running
	err := setJobPhase(job.Name, job.Namespace, khjobv1.JobRunning)
	if err != nil {
		log.Errorln("Error setting job phase:", err)
	}

	err = j.Run(ctx, kubernetesClient)
	if err != nil {
		log.Errorln("Error running job:", j.Name(), "in namespace", j.CheckNamespace()+":", err)
		if strings.Contains(err.Error(), "pod deleted expectedly") {
			log.Infoln("Skipping this job due to expected pod removal before completion")
		}
		// set any job run errors in the CRD
		err = k.setJobExecutionError(j.Name(), j.CheckNamespace(), err)
		if err != nil {
			log.Errorln("Error setting job execution error:", err)
		}
		// exit out of this runJob
		return
	}
	log.Debugln("Done running job:", j.Name(), "in namespace", j.CheckNamespace())

	// Record job run end time
	// Subtract 10 seconds from run time since there are two 5 second sleeps during the job run where kuberhealthy
	// waits for all pods to clear before running the check and waits for all pods to exit once the check has finished
	// running. Both occur before and after the kh job pod completes its run.
	jobRunDuration := time.Since(jobStartTime) - time.Second*10

	// make a new state for this job and fill it from the job's current status
	jobDetails, err := getJobState(j)
	if err != nil {
		log.Errorln("Error setting check state after run:", j.Name(), "in namespace", j.CheckNamespace()+":", err)
	}
	details := khstatev1.NewWorkloadDetails(khstatev1.KHJob)
	details.Namespace = j.CheckNamespace()
	details.OK, details.Errors = j.CurrentStatus()
	details.RunDuration = jobRunDuration.String()
	details.CurrentUUID = jobDetails.CurrentUUID

	// Fetch node information from running check pod using kh run uuid
	selector := "kuberhealthy-run-id=" + details.CurrentUUID
	pod, err := k.fetchPodBySelector(ctx, selector)
	if err != nil {
		log.Errorln(err)
	}
	details.Node = pod.Spec.NodeName

	log.Debugln("node name:", details.Node, "nodeName", j.Node)

	// send data to the metric forwarder if configured
	if k.MetricForwarder != nil {
		checkStatus := 0
		if details.OK {
			checkStatus = 1
		}

		runDuration, err := time.ParseDuration(details.RunDuration)
		if err != nil {
			log.Errorln("Error parsing run duration", err)
		}

		tags := map[string]string{
			"KuberhealthyPod": details.AuthoritativePod,
			"Namespace":       j.CheckNamespace(),
			"Name":            j.Name(),
			"Errors":          strings.Join(details.Errors, ","),
		}
		metric := metrics.Metric{
			{j.Name() + "." + j.CheckNamespace(): checkStatus},
			{"RunDuration." + j.Name() + "." + j.CheckNamespace(): runDuration.Seconds()},
		}
		err = k.MetricForwarder.Push(metric, tags)
		if err != nil {
			log.Errorln("Error forwarding metrics", err)
		}
	}

	log.Infoln("Setting state of job", j.Name(), "in namespace", j.CheckNamespace(), "to", details.OK, details.Errors, details.RunDuration, details.CurrentUUID, details.GetKHWorkload())

	// store the job state with the CRD
	err = k.storeCheckState(j.Name(), j.CheckNamespace(), details)
	if err != nil {
		log.Errorln("Error storing CRD state for job:", j.Name(), "in namespace", j.CheckNamespace(), err)
	}

	// set KHJob phase to running:
	err = setJobPhase(j.Name(), j.CheckNamespace(), khjobv1.JobCompleted)
	if err != nil {
		log.Errorln("Error setting job phase:", err)
	}
}

// runCheck runs a check on an interval and sets its status each run
func (k *Kuberhealthy) runCheck(ctx context.Context, c *external.Checker) {

	log.Println("Starting check:", c.CheckNamespace(), "/", c.Name())

	// run on an interval specified by the package
	ticker := time.NewTicker(c.Interval())

	// run the check forever and write its results to the kuberhealthy
	// CRD resource for the check
	for {

		// break out if context cancels
		select {
		case <-ctx.Done():
			// we don't need to call a check shutdown here because the same func that cancels this context calls
			// shutdown on all the checks configured in the kuberhealthy struct.
			log.Infoln("Shutting down check run due to context cancellation:", c.Name(), "in namespace", c.CheckNamespace())
			return
		default:
		}

		// Run the check
		log.Infoln("Running check:", c.Name())
		// Record check run start time
		checkStartTime := time.Now()
		err := c.Run(ctx, kubernetesClient)
		if err != nil {
			log.Errorln("Error running check:", c.Name(), "in namespace", c.CheckNamespace()+":", err)
			if strings.Contains(err.Error(), "pod deleted expectedly") {
				log.Infoln("Skipping this run due to expected pod removal before completion")
				<-ticker.C
			}
			// set any check run errors in the CRD
			err = k.setCheckExecutionError(c.Name(), c.CheckNamespace(), err)
			if err != nil {
				log.Errorln("Error setting check execution error:", err)
			}
			<-ticker.C
			continue
		}
		log.Debugln("Done running check:", c.Name(), "in namespace", c.CheckNamespace())

		// Record check run end time
		// Subtract 10 seconds from run time since there are two 5 second sleeps during the check run where kuberhealthy
		// waits for all pods to clear before running the check and waits for all pods to exit once the check has finished
		// running. Both occur before and after the checker pod completes its run.
		checkRunDuration := time.Since(checkStartTime) - time.Second*10

		// make a new state for this check and fill it from the check's current status
		checkDetails, err := getCheckState(c)
		if err != nil {
			log.Errorln("Error setting check state after run:", c.Name(), "in namespace", c.CheckNamespace()+":", err)
		}
		details := khstatev1.NewWorkloadDetails(khstatev1.KHCheck)
		details.Namespace = c.CheckNamespace()
		details.OK, details.Errors = c.CurrentStatus()
		details.RunDuration = checkRunDuration.String()
		details.CurrentUUID = checkDetails.CurrentUUID

		// Fetch node information from running check pod using kh run uuid
		selector := "kuberhealthy-run-id=" + details.CurrentUUID
		pod, err := k.fetchPodBySelector(ctx, selector)
		if err != nil {
			log.Errorln(err)
		}
		details.Node = pod.Spec.NodeName

		log.Debugln("node name:", details.Node, "nodeName", c.Node)

		// send data to the metric forwarder if configured
		if k.MetricForwarder != nil {
			checkStatus := 0
			if details.OK {
				checkStatus = 1
			}

			runDuration, err := time.ParseDuration(details.RunDuration)
			if err != nil {
				log.Errorln("Error parsing run duration", err)
			}

			tags := map[string]string{
				"KuberhealthyPod": details.AuthoritativePod,
				"Namespace":       c.CheckNamespace(),
				"Name":            c.Name(),
				"Errors":          strings.Join(details.Errors, ","),
			}
			metric := metrics.Metric{
				{c.Name() + "." + c.CheckNamespace(): checkStatus},
				{"RunDuration." + c.Name() + "." + c.CheckNamespace(): runDuration.Seconds()},
			}
			err = k.MetricForwarder.Push(metric, tags)
			if err != nil {
				log.Errorln("Error forwarding metrics", err)
			}
		}

		log.Infoln("Setting state of check", c.Name(), "in namespace", c.CheckNamespace(), "to", details.OK, details.Errors, details.RunDuration, details.CurrentUUID, details.GetKHWorkload())

		// store the check state with the CRD
		err = k.storeCheckState(c.Name(), c.CheckNamespace(), details)
		if err != nil {
			log.Errorln("Error storing CRD state for check:", c.Name(), "in namespace", c.CheckNamespace(), err)
		}

		log.Infoln("Waiting for next run of check", c.Name(), "in namespace", c.CheckNamespace())
		<-ticker.C // wait for next run
	}
}

// storeCheckState stores the check state in its cluster CRD
func (k *Kuberhealthy) storeCheckState(checkName string, checkNamespace string, details khstatev1.WorkloadDetails) error {

	// ensure the CRD resource exits
	err := ensureStateResourceExists(checkName, checkNamespace, details.GetKHWorkload())
	if err != nil {
		return err
	}

	// put the status on the CRD from the check
	err = setCheckStateResource(checkName, checkNamespace, details)

	//TODO: Make this retry of updating custom resources repeatable
	//
	// We commonly see a race here with the following type of error:
	// "Error storing CRD state for check: pod-restarts in namespace kuberhealthy Operation cannot be fulfilled on khstates.comcast.github.io \"pod-restarts\": the object
	// has been modified; please apply your changes to the latest version and try again"
	//
	// If we see this error, we fetch the updated object, re-apply our changes, and try again
	delay := time.Duration(time.Second * 1)
	maxTries := 7
	tries := 0
	for err != nil && strings.Contains(err.Error(), "the object has been modified") {

		// if too many retires have occurred, we fail up the stack further
		if tries > maxTries {
			return fmt.Errorf("failed to update khstate for check %s in namespace %s after %d with error %w", checkName, checkNamespace, maxTries, err)
		}
		log.Infoln("Failed to update khstate for check because object was modified by another process.  Retrying in " + delay.String() + ".  Try " + strconv.Itoa(tries) + " of " + strconv.Itoa(maxTries) + ".")

		// sleep and double the delay between checks (exponential backoff)
		time.Sleep(delay)
		delay = delay + delay

		// try setting the check state again
		err = setCheckStateResource(checkName, checkNamespace, details)

		// count how many times we've retried
		tries++
	}

	return err
}

// StartWebServer starts a JSON status web server at the specified listener.
func (k *Kuberhealthy) StartWebServer() {
	log.Infoln("Configuring web server")
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		err := k.prometheusMetricsHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// Accept status reports coming from external checker pods
	http.HandleFunc("/externalCheckStatus", func(w http.ResponseWriter, r *http.Request) {
		err := k.externalCheckReportHandler(w, r)
		if err != nil {
			log.Errorln("externalCheckStatus endpoint error:", err)
		}
	})

	// Assign all requests to be handled by the healthCheckHandler function
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := k.healthCheckHandler(w, r)
		if err != nil {
			log.Errorln(err)
		}
	})

	// start web server any time it exits
	for {
		log.Infoln("Starting web services on port", k.ListenAddr)
		err := http.ListenAndServe(k.ListenAddr, nil)
		if err != nil {
			log.Errorln("Web server ERROR:", err)
		}
		time.Sleep(time.Second / 2)
	}
}

// PodReportInfo holds info about an incoming IP to the external check reporting endpoint
type PodReportInfo struct {
	Name      string
	UUID      string
	Namespace string
}

// validateExternalRequest calls the Kubernetes API to fetch details about a pod using a selector string.
// It validates that the pod is allowed to report the status of a check. The pod is also expected
// to have the environment variable KH_CHECK_NAME
func (k *Kuberhealthy) validateExternalRequest(ctx context.Context, selector string) (PodReportInfo, error) {

	var podUUID string
	var podCheckName string
	var podCheckNamespace string

	reportInfo := PodReportInfo{}

	// fetch the pod from the api using a specified selector. We keep retrying for some time to avoid kubernetes control
	// plane api race conditions wherein fast reporting pods are not found in pod listings
	pod, err := k.fetchPodBySelectorForDuration(ctx, selector, time.Minute)
	if err != nil {
		return reportInfo, err
	}

	// set the pod namespace and name from the returned metadata
	podCheckName = pod.Annotations[KHCheckNameAnnotationKey]
	if len(podCheckName) == 0 {
		return reportInfo, errors.New("error finding check name annotation on calling pod with selector: " + selector)
	}

	podCheckNamespace = pod.GetNamespace()
	log.Debugln("Found check named", podCheckName, "in namespace", podCheckNamespace)

	// pile up all the env vars for searching
	var envVars []v1.EnvVar
	// we found our pod, lets return all its env vars from all its containers
	for _, container := range pod.Spec.Containers {
		envVars = append(envVars, container.Env...)
	}

	log.Debugln("Env vars found on pod with selector", selector, envVars)

	// validate that the environment variables we expect are in place based on the check's name and UUID
	// compared to what is in the khcheck custom resource
	var foundUUID bool
	for _, e := range envVars {
		log.Debugln("Checking environment variable on calling pod:", e.Name, e.Value)
		if e.Name == external.KHRunUUID {
			log.Debugln("Found value on calling pod", selector, "value:", external.KHRunUUID, e.Value)
			podUUID = e.Value
			foundUUID = true
		}
	}

	// verify that we found the UUID
	if !foundUUID {
		return reportInfo, errors.New("error finding environment variable on remote pod: " + external.KHRunUUID)
	}

	// we know that we have a UUID and check name now, so lets check their validity.  First, we sanity check.
	if len(podCheckName) < 1 {
		return reportInfo, errors.New("pod check name was invalid or unset")
	}
	if len(podCheckNamespace) < 1 {
		return reportInfo, errors.New("pod check namespace was invalid or unset")
	}
	if len(podUUID) < 1 {
		return reportInfo, errors.New("pod uuid was invalid or unset")
	}

	// create a report to send back to the function invoker
	reportInfo.Name = podCheckName
	reportInfo.Namespace = podCheckNamespace
	reportInfo.UUID = podUUID

	// next, we check the uuid against the check name to see if this uuid is the expected one.  if it isn't,
	// we return an error
	whitelisted, err := k.isUUIDWhitelistedForCheck(podCheckName, podCheckNamespace, podUUID)
	if err != nil {
		return reportInfo, fmt.Errorf("failed to fetch whitelisted UUID for check with error: %w", err)
	}
	if !whitelisted {
		return reportInfo, errors.New("pod was not properly whitelisted for reporting status of check " + podCheckName + " with uuid " + podUUID + " and namespace " + podCheckNamespace)
	}

	return reportInfo, nil
}

// fetchPodBySelectorForDuration attempts to fetch a pod by a specified selector repeatedly for the supplied duration.
// If the pod is found, then we return it.  If the pod is not found after the duration, we return an error
func (k *Kuberhealthy) fetchPodBySelectorForDuration(ctx context.Context, selector string, d time.Duration) (v1.Pod, error) {
	endTime := time.Now().Add(d)

	for {
		if time.Now().After(endTime) {
			return v1.Pod{}, errors.New("Failed to fetch source pod with selector " + selector + " after trying for " + d.String())
		}

		p, err := k.fetchPodBySelector(ctx, selector)
		if err != nil {
			log.Warningln("was unable to find calling pod with selector " + selector + " while watching for duration. Error: " + err.Error())
			time.Sleep(time.Second)
			continue
		}

		return p, err
	}
}

// fetchPodBySelector fetches the pod by it's `kuberhealthy-run-id` label selector or by its `status.podIP` field selector
func (k *Kuberhealthy) fetchPodBySelector(ctx context.Context, selector string) (v1.Pod, error) {
	var pod v1.Pod

	podClient := kubernetesClient.CoreV1().Pods(k.TargetNamespace)

	// Use either label selector or field selector depending on the selector string passed through
	// LabelSelector: "kuberhealthy-run-id=" + uuid,
	// FieldSelector: "status.podIP==" + remoteIP + ",status.phase==Running",
	var listOptions metav1.ListOptions
	if strings.Contains(selector, "kuberhealthy-run-id") {
		listOptions = metav1.ListOptions{
			LabelSelector: selector,
		}
	}

	if strings.Contains(selector, "status.podIP") {
		listOptions = metav1.ListOptions{
			FieldSelector: selector,
		}
	}

	podList, err := podClient.List(ctx, listOptions)
	if err != nil {
		return pod, errors.New("failed to fetch pod with selector " + selector + " with error: " + err.Error())
	}

	// ensure that we only got back one pod, because two means something awful has happened and 0 means we
	// didnt find one
	if len(podList.Items) == 0 {
		return pod, errors.New("failed to find a pod with selector " + selector)
	}
	if len(podList.Items) > 1 {
		return pod, errors.New("failed to fetch pod with selector " + selector + " - found two or more with same label")
	}

	// check if the pod has containers
	if len(podList.Items[0].Spec.Containers) == 0 {
		return pod, errors.New("failed to fetch environment variables from pod with selector" + selector + " - pod had no containers")
	}

	return podList.Items[0], nil
}

func (k *Kuberhealthy) externalCheckReportHandlerLog(s ...interface{}) {
	log.Infoln(s...)
}

// validatePodReportBySourceIP gets the header `kh-run-uuid` value from the request and forms a selector with it to
// validate that the request is coming from a kuberhealthy check pod
func (k *Kuberhealthy) validateUsingRequestHeader(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {

	var podReport PodReportInfo
	var err error
	if len(r.Header.Get("kh-run-uuid")) == 0 {
		return podReport, false, nil
	}
	selector := "kuberhealthy-run-id=" + r.Header.Get("kh-run-uuid")
	podReport, err = k.validateExternalRequest(ctx, selector)
	if err != nil {
		return podReport, false, err
	}
	return podReport, true, nil
}

// validatePodReportBySourceIP parses the remoteAddr from the request and forms a selector with the remote IP to
// validate that the request is coming from a kuberhealthy check pod
func (k *Kuberhealthy) validatePodReportBySourceIP(ctx context.Context, r *http.Request) (PodReportInfo, error) {

	var podReport PodReportInfo
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return podReport, err
	}
	selector := "status.podIP==" + ip + ",status.phase==Running"
	podReport, err = k.validateExternalRequest(ctx, selector)
	if err != nil {
		return podReport, err
	}
	return podReport, nil
}

// externalCheckReportHandler handles requests coming from external checkers reporting their status.
// This endpoint checks that the external check report is coming from the correct UUID or pod IP before recording
// the reported status of the corresponding external check.  This endpoint expects a JSON payload of
// the `State` struct found in the github.com/kuberhealthy/kuberhealthy/v2/pkg/health package.  The request
// causes a check of the calling pod's spec via the API to ensure that the calling pod is expected
// to be reporting its status.
func (k *Kuberhealthy) externalCheckReportHandler(w http.ResponseWriter, r *http.Request) error {
	// make a request ID for tracking this request
	requestID := "web: " + uuid.New().String()

	ctx := r.Context()

	k.externalCheckReportHandlerLog(requestID, "Client connected to check report handler from", r.UserAgent())

	// Validate request using the kh-run-uuid header. If the header doesn't exist, or there's an error with validation,
	// validate using the pod's remote IP.
	k.externalCheckReportHandlerLog(requestID, "validating external check status report from its reporting kuberhealthy run uuid:", r.Header.Get("kh-run-uuid"))
	podReport, reportValidated, err := k.validateUsingRequestHeader(ctx, r)
	if err != nil {
		k.externalCheckReportHandlerLog(requestID, "Failed to look up pod by its kh-run-uuid header:", r.Header.Get("kh-run-uuid"), err)
	}

	// If the check uuid header is missing, attempt to validate using calling pod's source IP
	if !reportValidated {
		k.externalCheckReportHandlerLog(requestID, "validating external check status report from the pod's remote IP:", r.RemoteAddr)
		podReport, err = k.validatePodReportBySourceIP(ctx, r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			k.externalCheckReportHandlerLog(requestID, "Failed to look up pod by its IP:", r.RemoteAddr, err)
			return nil
		}
	}
	k.externalCheckReportHandlerLog(requestID, "Calling pod is", podReport.Name, "in namespace", podReport.Namespace)

	// append pod info to request id for easy check tracing in logs
	requestID = requestID + " (" + podReport.Namespace + "/" + podReport.Name + ")"

	// ensure the client is sending a valid payload in the request body
	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		k.externalCheckReportHandlerLog(requestID, "Failed to read request body:", err.Error(), r.RemoteAddr)
		return nil
	}
	log.Debugln("Check report body:", string(b))

	// decode the bytes into a status struct as used by the client
	state := status.Report{}
	err = json.Unmarshal(b, &state)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		k.externalCheckReportHandlerLog(requestID, "Failed to unmarshal state json:", err, r.RemoteAddr)
		return nil
	}
	log.Debugf("Check report after unmarshal: +%v\n", state)

	// ensure that if ok is set to false, then an error is provided
	if !state.OK {
		if len(state.Errors) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			k.externalCheckReportHandlerLog(requestID, "Client attempted to report OK false without any error strings")
			return nil
		}
		for _, e := range state.Errors {
			if len(e) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				k.externalCheckReportHandlerLog(requestID, "Client attempted to report a blank error string")
				return nil
			}
		}
	}

	checkRunDuration := time.Duration(0).String()
	khWorkload := determineKHWorkload(podReport.Name, podReport.Namespace)

	switch khWorkload {
	case khstatev1.KHCheck:
		checkDetails := k.stateReflector.CurrentStatus().CheckDetails
		checkRunDuration = checkDetails[podReport.Namespace+"/"+podReport.Name].RunDuration
	case khstatev1.KHJob:
		jobDetails := k.stateReflector.CurrentStatus().JobDetails
		checkRunDuration = jobDetails[podReport.Namespace+"/"+podReport.Name].RunDuration
	}

	// create a details object from our incoming status report before storing it as a khstate custom resource
	details := khstatev1.NewWorkloadDetails(khWorkload)
	details.Errors = state.Errors
	details.OK = state.OK
	details.RunDuration = checkRunDuration
	details.Namespace = podReport.Namespace
	details.CurrentUUID = podReport.UUID

	// since the check is validated, we can proceed to update the status now
	k.externalCheckReportHandlerLog(requestID, "Setting check with name", podReport.Name, "in namespace", podReport.Namespace, "to 'OK' state:", details.OK, "uuid", details.CurrentUUID, details.GetKHWorkload())
	err = k.storeCheckState(podReport.Name, podReport.Namespace, details)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		k.externalCheckReportHandlerLog(requestID, "failed to store check state for %s: %w", podReport.Name, err)
		return fmt.Errorf("failed to store check state for %s: %w", podReport.Name, err)
	}

	// write ok back to caller
	w.WriteHeader(http.StatusOK)
	k.externalCheckReportHandlerLog(requestID, "Request completed successfully.")
	return nil
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
	log.Infoln("Client connected to prometheus metrics endpoint from", r.RemoteAddr, r.UserAgent())
	state := k.getCurrentState([]string{})

	m := metrics.GenerateMetrics(state, cfg.PromMetricsConfig)
	// write summarized health check results back to caller
	_, err := w.Write([]byte(m))
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}

// healthCheckHandler returns the current status of checks loaded into Kuberhealthy
// as JSON to the client. Respects namespace requests via URL query parameters (i.e. /?namespace=default)
func (k *Kuberhealthy) healthCheckHandler(w http.ResponseWriter, r *http.Request) error {
	log.Infoln("Client connected to status page from", r.RemoteAddr, r.UserAgent())

	// If a request body was supplied, throw an error to ensure that checks don't report into the wrong url
	body, err := io.ReadAll(r.Body)
	if len(body) > 0 {
		log.Warningln("Unexpected body from status page request. Verify check is reporting to the right status url", r.RemoteAddr, r.RequestURI)
		w.WriteHeader(http.StatusBadRequest)
		return err
	}

	// get URL query parameters if there are any
	values := r.URL.Query()
	namespaceValue := values.Get("namespace")
	// .Get() will return an "" if there is no value associated -- we do not want to pass "" as a requested namespace
	var namespaces []string
	if len(namespaceValue) != 0 {
		namespaceSplits := strings.Split(namespaceValue, ",")
		// a query like (/?namespace=,) will cause .Split() to return an array of two empty strings ["", ""]
		// so we need to filter those out
		for _, namespaceSplit := range namespaceSplits {
			if len(namespaceSplit) != 0 {
				namespaces = append(namespaces, namespaceSplit)
			}
		}
	}

	// fetch the current status from our khstate resources
	state := k.getCurrentState(namespaces)

	// write summarized health check results back to caller
	err = state.WriteHTTPStatusResponse(w)
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}

// getCurrentState fetches the current state of all checks from requested namespaces
// their CRD objects and returns the summary as a health.State. Without a requested namespace,
// this will return the state of ALL found checks.
// Failures to fetch CRD state return an error.
func (k *Kuberhealthy) getCurrentState(namespaces []string) health.State {

	currentMaster, err := masterCalculation.CalculateMaster(kubernetesClient)
	if err != nil {
		log.Errorln("Failed to calculate master:", err)
	}

	var currentState health.State
	if len(namespaces) != 0 {
		currentState = k.getCurrentStatusForNamespaces(namespaces)
	} else {
		currentState = k.stateReflector.CurrentStatus()
	}

	currentState.CurrentMaster = currentMaster
	if len(cfg.StateMetadata) != 0 {
		currentState.Metadata = cfg.StateMetadata
	}

	return currentState
}

// getCurrentState fetches the current state of all checks from the requested namespaces
// their CRD objects and returns the summary as a health.State.
// Failures to fetch CRD state return an error.
func (k *Kuberhealthy) getCurrentStatusForNamespaces(namespaces []string) health.State {
	// if there is are requested namespaces, then filter out checks from namespaces not matching those requested
	states := k.stateReflector.CurrentStatus()
	statesForNamespaces := states
	statesForNamespaces.Errors = []string{}
	statesForNamespaces.OK = true
	statesForNamespaces.CheckDetails = make(map[string]khstatev1.WorkloadDetails)
	statesForNamespaces.JobDetails = make(map[string]khstatev1.WorkloadDetails)
	if len(namespaces) != 0 {
		statesForNamespaces = validateCurrentStatusForNamespaces(states.CheckDetails, namespaces, statesForNamespaces, khstatev1.KHCheck)
		statesForNamespaces = validateCurrentStatusForNamespaces(states.JobDetails, namespaces, statesForNamespaces, khstatev1.KHJob)
	}

	log.Infoln("khState reflector returning current status on", len(statesForNamespaces.CheckDetails), "check khStates and", len(statesForNamespaces.JobDetails), "job khStates")
	return statesForNamespaces
}

// validateCurrentStatusForNamespaces ranges through all CheckDetails or JobDetails to store in a new health state for namespaces
func validateCurrentStatusForNamespaces(details map[string]khstatev1.WorkloadDetails, namespaces []string, statesForNamespaces health.State, workload khstatev1.KHWorkload) health.State {

	for checkName, checkState := range details {
		// check if the namespace matches anything requested
		if !containsString(checkState.Namespace, namespaces) {
			log.Debugln("Skipping", checkName, "because it is not from the", namespaces, "namespace(s)")
			continue
		}

		// skip the check if it has never been run before.  This prevents checks that have not yet
		// run from showing in the status page.
		if len(checkState.AuthoritativePod) == 0 {
			log.Debugln("Output for", checkName, checkState.Namespace, "hidden from status page due to blank authoritative pod")
			continue
		}

		// parse check status from CRD and add it to the global status of errors. Skip blank errors
		for _, e := range checkState.Errors {
			if len(strings.TrimSpace(e)) == 0 {
				log.Warningln("Skipped an error that was blank when adding check details to current state.")
				continue
			}
			statesForNamespaces.AddError(e)
			log.Debugln("Status page: Setting global OK state to false due to check details not being OK")
			statesForNamespaces.OK = false
		}

		// update details struct
		switch workload {
		case khstatev1.KHCheck:
			statesForNamespaces.CheckDetails[checkName] = checkState
		case khstatev1.KHJob:
			statesForNamespaces.JobDetails[checkName] = checkState
		}
	}

	return statesForNamespaces
}

// getCheck returns a Kuberhealthy check object from its name, returns an error otherwise
func (k *Kuberhealthy) getCheck(name string, namespace string) (*external.Checker, error) {
	for _, c := range k.Checks {
		if c.Name() == name && c.CheckNamespace() == namespace {
			return c, nil
		}
	}
	return &external.Checker{}, fmt.Errorf("could not find Kuberhealthy check with name %s", name)
}

// getJob returns a Kuberhealthy job object from its name, returns an error otherwise
func (k *Kuberhealthy) getJob(name string, namespace string) (*external.Checker, error) {

	var kjob external.Checker
	j, err := khJobClient.KuberhealthyJobs(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		log.Debugln("Error getting khjob:", name, err)
		return &kjob, err
	}

	return k.configureJob(j), nil
}

// configureChecks removes all checks set in Kuberhealthy and reloads them
// based on the configuration options
func (k *Kuberhealthy) configureChecks(ctx context.Context) {
	log.Infoln("control: Loading check configuration...")

	// wipe all existing checks before we configure
	k.Checks = []*external.Checker{}

	// check external check configurations
	err := k.addExternalChecks(ctx)
	if err != nil {
		log.Errorln("control: ERROR loading external checks:", err)
	}
}

// isUUIDWhitelistedForCheck determines if the supplied uuid is whitelisted for the
// check with the supplied name.  Only one UUID can be whitelisted at a time.
// Operations are not atomic.  Whitelisting prevents expired or invalidated pods from
// reporting into the status endpoint when they shouldn't be.
func (k *Kuberhealthy) isUUIDWhitelistedForCheck(checkName string, checkNamespace string, uuid string) (bool, error) {

	// get the item in question
	checkState, err := khStateClient.KuberhealthyStates(checkNamespace).Get(checkName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	log.Debugln("Validating current UUID", checkState.Spec.CurrentUUID, "vs incoming UUID:", uuid)
	if checkState.Spec.CurrentUUID == uuid {
		return true, nil
	}
	return false, nil
}

// configureInfluxForwarding sets up initial influxdb metric sending
func (k *Kuberhealthy) configureInfluxForwarding() {

	// configure influxdb
	metricClient, err := configureInflux()
	if err != nil {
		log.Fatalln("Error setting up influx client:", err)
	}
	k.MetricForwarder = metricClient
}

// func listUnstructuredKHChecks(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {

// 	khCheckGroupVersionResource := schema.GroupVersionResource{
// 		Version:  checkCRDVersion,
// 		Resource: checkCRDResource,
// 		Group:    checkCRDGroup,
// 	}

// 	unstructuredList, err := dynamicClient.Resource(khCheckGroupVersionResource).Namespace(namespace).List(ctx, metav1.ListOptions{})
// 	if err != nil {
// 		return unstructuredList, err
// 	}

// 	return unstructuredList, err
// }

// func convertUnstructuredKhCheck(unstructured unstructured.Unstructured) (khcheckv1.KuberhealthyCheck, error) {
// 	un := unstructured.UnstructuredContent()
// 	var khCheck khcheckv1.KuberhealthyCheck
// 	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un, &khCheck)
// 	if err != nil {
// 		return khCheck, fmt.Errorf("error converting unstructured object to khcheck: %w", err)
// 	}

// 	return khCheck, err
// }

// func watchUnstructuredKHChecks(ctx context.Context, namespace string) (watch.Interface, error) {

// 	khCheckGroupVersionResource := schema.GroupVersionResource{
// 		Version:  checkCRDVersion,
// 		Resource: checkCRDResource,
// 		Group:    checkCRDGroup,
// 	}

// 	watcher, err := dynamicClient.Resource(khCheckGroupVersionResource).Namespace(namespace).Watch(ctx, metav1.ListOptions{})
// 	if err != nil {
// 		return watcher, err
// 	}

// 	return watcher, err
// }
