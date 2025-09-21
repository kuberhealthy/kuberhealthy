package kuberhealthy

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/envs"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicinformer "k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	record "k8s.io/client-go/tools/record"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	restconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// defaultRunInterval is used when a check does not specify a runInterval or it fails to parse.
	defaultRunInterval = time.Minute * 10
	// defaultRunTimeout is the amount of time a pod is allowed to run before it is considered timed out.
	defaultRunTimeout = 30 * time.Second
	// scheduleLoopInterval controls how often Kuberhealthy scans for checks to run.
	scheduleLoopInterval = 30 * time.Second
	checkLabel           = "khcheck"
	runUUIDLabel         = "kh-run-uuid"
	// timeoutGracePeriod adds a tiny buffer before flagging a run as failed so pods can finish cleanly at the
	// deadline without tripping a race.
	timeoutGracePeriod = 2 * time.Second
	// defaultFailedPodRetentionDays is the number of days to retain failed pods before they are reaped.
	defaultFailedPodRetentionDays = 4
	// defaultMaxFailedPods is the maximum number of failed pods to retain for a check.
	defaultMaxFailedPods = 5
)

var kuberhealthyCheckGVR = khapi.GroupVersion.WithResource("kuberhealthychecks")

// Kuberhealthy handles background processing for checks
type Kuberhealthy struct {
	Context     context.Context
	cancel      context.CancelFunc
	running     bool          // indicates that Start() has been called and this instance is running
	CheckClient client.Client // Kubernetes client for check CRUD
	restConfig  *rest.Config  // cached config for building informers once start() wires everything together

	Recorder record.EventRecorder // emits k8s events for khcheck lifecycle

	ReportingURL string

	loopMu      sync.Mutex
	loopRunning bool
	doneChan    chan struct{} // signaled when shutdown completes

}

// New creates a new Kuberhealthy instance, event recorder, and optional shutdown notifier.
// The shutdown channel can be omitted or passed as nil if not needed.
func New(ctx context.Context, checkClient client.Client, doneChan ...chan struct{}) *Kuberhealthy {
	log.Infoln("New Kuberhealthy instance created")

	var recorder record.EventRecorder

	cfg, err := restconfig.GetConfig()
	if err != nil {
		log.Errorln("event recorder disabled:", err)
	} else {
		cs, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			log.Errorln("event recorder disabled:", err)
		} else {
			broadcaster := record.NewBroadcaster()
			broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: cs.CoreV1().Events("")})
			if err := khapi.AddToScheme(k8scheme.Scheme); err != nil {
				log.Errorln("event recorder disabled:", err)
			} else {
				recorder = broadcaster.NewRecorder(k8scheme.Scheme, corev1.EventSource{Component: "kuberhealthy"})
			}
		}
	}

	var ch chan struct{}
	if len(doneChan) > 0 {
		ch = doneChan[0]
	}

	return &Kuberhealthy{
		Context:     ctx,
		CheckClient: checkClient,
		Recorder:    recorder,
		doneChan:    ch,
	}
}

// SetCheckClient sets the controller's kube client for API operatons against the control plane. This must
// bet set before Start() is invoked, but can not be in the constructor because of an interdependency between
// the controller embedding the Kuberhealthy sturuct and the Kuberhealthy struct needing a client.Client.
// Using the same client.Client concurrently can cause rare memory access race conditions.
func (kh *Kuberhealthy) SetCheckClient(checkClient client.Client) {
	kh.CheckClient = checkClient
}

// SetReportingURL configures the endpoint check pods should report status to.
func (kh *Kuberhealthy) SetReportingURL(url string) {
	kh.ReportingURL = url
}

// Start begins background processing for Kuberhealthy checks.
// A Kubernetes rest.Config is optional; when provided, khchecks will be
// watched for creation, update, and deletion events.
func (kh *Kuberhealthy) Start(ctx context.Context, cfg *rest.Config) error {
	if kh.IsStarted() {
		return fmt.Errorf("error: kuberhealthy main controller was started but it was already running")
	}

	if kh.CheckClient == nil {
		return fmt.Errorf("error: kuberhealthy main controller was started but it did not have a check client set. Use SetClient to set.")
	}

	kh.Context, kh.cancel = context.WithCancel(ctx)
	kh.running = true
	if kh.doneChan != nil {
		go func() {
			<-kh.Context.Done()
			kh.running = false
			kh.doneChan <- struct{}{}
		}()
	}
	go kh.startScheduleLoop()
	go kh.runReaper(kh.Context, time.Minute)
	resumeErr := kh.resumeCheckTimeouts()
	if resumeErr != nil {
		log.Errorln("failed to resume running check timeouts:", resumeErr)
	}
	kh.restConfig = cfg
	if kh.restConfig != nil {
		go kh.startKHCheckWatch()
	}

	log.Infoln("Kuberhealthy start-up complete.")
	return nil
}

// Stop halts background scheduling and marks the instance as no longer running.
func (kh *Kuberhealthy) Stop() {
	if !kh.IsStarted() {
		return
	}
	if kh.cancel != nil {
		kh.cancel()
	}
	kh.running = false
}

// setLoopRunning sets the loopRunning flag in a threadsafe manner. When setting
// to true, it returns false if the loop was already running to prevent
// duplicates. When setting to false, it always returns true.
func (kh *Kuberhealthy) setLoopRunning(running bool) bool {
	kh.loopMu.Lock()
	defer kh.loopMu.Unlock()
	if running && kh.loopRunning {
		return false
	}
	kh.loopRunning = running
	return true
}

// startScheduleLoop periodically evaluates all known khchecks and starts new runs when due.
func (kh *Kuberhealthy) startScheduleLoop() {
	if !kh.setLoopRunning(true) {
		return
	}
	defer kh.setLoopRunning(false)

	ticker := time.NewTicker(scheduleLoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-kh.Context.Done():
			return
		case <-ticker.C:
			kh.scheduleChecks()
		}
	}
}

// scheduleChecks iterates through all khchecks and starts any that are due to run.
func (kh *Kuberhealthy) scheduleChecks() {
	uList := &unstructured.UnstructuredList{}
	uList.SetGroupVersionKind(khapi.GroupVersion.WithKind("KuberhealthyCheckList"))
	if err := kh.CheckClient.List(kh.Context, uList); err != nil {
		log.Errorln("failed to list khchecks:", err)
		return
	}

	for _, khcheck := range uList.Items {
		runInterval := kh.runIntervalForCheck(&khcheck)

		var check khapi.KuberhealthyCheck
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(khcheck.Object, &check); err != nil {
			log.Errorf("failed to convert check %s/%s: %v", khcheck.GetNamespace(), khcheck.GetName(), err)
			continue
		}
		check.EnsureCreationTimestamp()

		lastStart := time.Unix(check.Status.LastRunUnix, 0)

		// skip checks that are already running
		if check.CurrentUUID() != "" {
			continue
		}

		// wait until the run interval has elapsed before starting again
		remaining := runInterval - time.Since(lastStart)
		if remaining > 0 {
			continue
		}

		if err := kh.StartCheck(&check); err != nil {
			log.Errorf("failed to start check %s/%s: %v", check.Namespace, check.Name, err)
		}
	}
}

// runIntervalForCheck returns the configured run interval for a check, falling back to the default.
func (kh *Kuberhealthy) runIntervalForCheck(u *unstructured.Unstructured) time.Duration {
	if v, found, err := unstructured.NestedString(u.Object, "spec", "runInterval"); err == nil && found && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Errorf("invalid runInterval %q on check %s/%s", v, u.GetNamespace(), u.GetName())
	}
	return defaultRunInterval
}

// startKHCheckWatch begins watching KuberhealthyCheck resources and reacts to events.
func (kh *Kuberhealthy) startKHCheckWatch() {
	if kh.restConfig == nil {
		log.Errorln("kuberhealthy: start watch requested without a rest config")
		return
	}

	dyn, err := dynamic.NewForConfig(kh.restConfig)
	if err != nil {
		log.Errorln("kuberhealthy: failed to create dynamic client:", err)
		return
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dyn, 0, metav1.NamespaceAll, nil)
	inf := factory.ForResource(kuberhealthyCheckGVR).Informer()

	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			khc, err := convertToKHCheck(obj)
			if err != nil {
				log.Errorln("error:", err)
				return
			}
			kh.handleCreate(khc)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldCheck, err := convertToKHCheck(oldObj)
			if err != nil {
				log.Errorln("error:", err)
				return
			}
			newCheck, err := convertToKHCheck(newObj)
			if err != nil {
				log.Errorln("error:", err)
				return
			}
			kh.handleUpdate(oldCheck, newCheck)
		},
		DeleteFunc: func(obj interface{}) {
			khc, err := convertToKHCheck(obj)
			if err != nil {
				log.Errorln("error:", err)
				return
			}
			kh.handleDelete(khc)
		},
	})

	go inf.Run(kh.Context.Done())
}

func (kh *Kuberhealthy) handleCreate(khc *khapi.KuberhealthyCheck) {
	if err := kh.addFinalizer(kh.Context, khc); err != nil {
		log.Errorln("error:", err)
		return
	}

	if err := kh.StartCheck(khc); err != nil {
		log.Errorln("error:", err)
	}
}

func (kh *Kuberhealthy) handleUpdate(oldCheck *khapi.KuberhealthyCheck, newCheck *khapi.KuberhealthyCheck) {
	if !newCheck.GetDeletionTimestamp().IsZero() {
		if kh.hasFinalizer(newCheck) {
			if err := kh.StopCheck(newCheck); err != nil {
				log.Errorln("error:", err)
			}

			nn := types.NamespacedName{Namespace: newCheck.Namespace, Name: newCheck.Name}
			refreshed, err := khapi.GetCheck(kh.Context, kh.CheckClient, nn)
			if err != nil {
				log.Errorln("error:", err)
				return
			}

			if err := kh.deleteFinalizer(kh.Context, refreshed); err != nil {
				log.Errorln("error:", err)
			}
		}
		return
	}

	log.WithFields(log.Fields{
		"namespace": newCheck.Namespace,
		"name":      newCheck.Name,
	}).Info("modified checker pod")

	if err := kh.UpdateCheck(oldCheck, newCheck); err != nil {
		log.Errorln("error:", err)
	}
}

func (kh *Kuberhealthy) handleDelete(khc *khapi.KuberhealthyCheck) {
	if kh.hasFinalizer(khc) {
		if err := kh.StopCheck(khc); err != nil {
			log.Errorln("error:", err)
		}

		nn := types.NamespacedName{Namespace: khc.Namespace, Name: khc.Name}
		refreshed, err := khapi.GetCheck(kh.Context, kh.CheckClient, nn)
		if err != nil {
			log.Errorln("error:", err)
			return
		}

		if err := kh.deleteFinalizer(kh.Context, refreshed); err != nil {
			log.Errorln("error:", err)
		}
		return
	}

	if err := kh.StopCheck(khc); err != nil {
		log.Errorln("error:", err)
	}
}

// resumeCheckTimeouts ensures active checks pick up timeout monitoring when Kuberhealthy restarts so that
// previously scheduled runs still fail at the correct time.
func (kh *Kuberhealthy) resumeCheckTimeouts() error {
	if kh.CheckClient == nil {
		return fmt.Errorf("check client not configured")
	}

	// pull every khcheck so we can resume timers for anything currently marked as running
	list := &khapi.KuberhealthyCheckList{}
	err := kh.CheckClient.List(kh.Context, list)
	if err != nil {
		return err
	}

	// iterate across the checks and restart a watcher for each active run
	for i := range list.Items {
		check := &list.Items[i]
		if check.CurrentUUID() == "" {
			// no pod is running for this check so there is nothing to resume
			continue
		}
		// restarting the watcher keeps pending timeouts accurate after controller restarts
		kh.startTimeoutWatcher(check)
	}

	return nil
}

// startTimeoutWatcher schedules a timeout evaluation for the provided check. The watcher runs in a separate
// goroutine so the caller does not block while waiting for the deadline.
func (kh *Kuberhealthy) startTimeoutWatcher(check *khapi.KuberhealthyCheck) {
	if check == nil {
		return
	}

	// only monitor checks that currently have a running pod
	uuid := check.CurrentUUID()
	if uuid == "" {
		return
	}

	// convert the stored start time to a Time value for deadline calculations
	startedAt := time.Unix(check.Status.LastRunUnix, 0)
	if startedAt.IsZero() {
		return
	}

	// respect the configured timeout for this check, falling back to the default when missing
	timeout := kh.checkTimeoutDuration(check)
	if timeout <= 0 {
		return
	}

	// capture identifiers we will need later inside the goroutine
	nn := types.NamespacedName{Namespace: check.Namespace, Name: check.Name}
	// derive the deadline for the run along with the two second grace period before we flag a timeout
	deadline := startedAt.Add(timeout)
	failAt := deadline.Add(timeoutGracePeriod)

	// watch the run in the background and mark it failed if the deadline passes without a report
	go kh.awaitTimeout(nn, uuid, startedAt, timeout, failAt)
}

// awaitTimeout waits for the grace period to elapse and then evaluates whether the run needs to be failed.
func (kh *Kuberhealthy) awaitTimeout(checkName types.NamespacedName, uuid string, startedAt time.Time, timeout time.Duration, failAt time.Time) {
	// wait until the timeout (plus grace period) elapses or the controller shuts down
	wait := time.Until(failAt)
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()

		select {
		case <-timer.C:
		case <-kh.Context.Done():
			return
		}
	}

	kh.failRunIfOverdue(checkName, uuid, startedAt, timeout)
}

// failRunIfOverdue refreshes the check and persists a timeout failure when the run still has the same UUID.
func (kh *Kuberhealthy) failRunIfOverdue(checkName types.NamespacedName, uuid string, startedAt time.Time, timeout time.Duration) {
	// reload the check to ensure we are acting on fresh state from the API server
	check, err := kh.readCheck(checkName)
	if err != nil {
		log.Errorf("timeout: failed to read check %s/%s: %v", checkName.Namespace, checkName.Name, err)
		return
	}

	// stop if a newer run has already started or the current run finished successfully
	if check.CurrentUUID() != uuid {
		return
	}

	// ensure the configured timeout truly elapsed in case clocks moved while we were waiting
	if time.Since(startedAt) < timeout {
		return
	}

	// craft human-readable messages for both the status field and the emitted event
	messageID := uuid
	if messageID == "" {
		messageID = "unknown"
	}
	started := "unknown"
	if !startedAt.IsZero() {
		started = startedAt.UTC().Format(time.RFC3339)
	}
	statusMessage := fmt.Sprintf("check run %s timed out after %s (started at %s)", messageID, timeout, started)
	eventMessage := fmt.Sprintf("check run %s exceeded timeout %s", messageID, timeout)

	// short circuit if the check already reflects this timeout so we avoid duplicate events
	alreadyTimedOut := len(check.Status.Errors) == 1 && check.Status.Errors[0] == statusMessage && !check.Status.OK
	if alreadyTimedOut {
		return
	}

	// persist the timeout result on the check status so the web UI immediately reflects the failure
	check.SetCheckExecutionError([]string{statusMessage})
	check.SetNotOK()

	err = khapi.UpdateCheck(kh.Context, kh.CheckClient, check)
	if err != nil {
		log.Errorf("timeout: failed updating check %s/%s timeout status: %v", check.Namespace, check.Name, err)
		return
	}

	// emit a warning event to surface the timeout in kubectl describe and other tooling
	if kh.Recorder != nil {
		kh.Recorder.Event(check, corev1.EventTypeWarning, "CheckRunTimedOut", eventMessage)
	}

	// clear the UUID so the scheduler can enqueue the next run
	clearErr := kh.clearUUID(checkName)
	if clearErr != nil {
		log.Errorf("timeout: failed to clear uuid for %s/%s: %v", checkName.Namespace, checkName.Name, clearErr)
	}
}

// recordPodCreationFailure refreshes the check status, records the pod creation error, and marks the run as failed.
func (kh *Kuberhealthy) recordPodCreationFailure(checkName types.NamespacedName, err error) error {
	// fetch the latest check to ensure we update the current status fields
	check, readErr := kh.readCheck(checkName)
	if readErr != nil {
		return readErr
	}

	// persist the creation failure on the status so operators immediately see the reason for the aborted run
	message := "check pod creation failed"
	if err != nil && err.Error() != "" {
		message = err.Error()
	}
	check.SetCheckExecutionError([]string{message})
	check.SetNotOK()

	// store the failure back to the API server
	return khapi.UpdateCheck(kh.Context, kh.CheckClient, check)
}

func (kh *Kuberhealthy) checkTimeoutDuration(check *khapi.KuberhealthyCheck) time.Duration {
	if check == nil {
		return 0
	}
	if check.Spec.Timeout != nil && check.Spec.Timeout.Duration > 0 {
		return check.Spec.Timeout.Duration
	}
	return defaultRunTimeout
}

// IsReportAllowed returns true when a checker pod is still permitted to report its result via the /check endpoint.
func (kh *Kuberhealthy) IsReportAllowed(check *khapi.KuberhealthyCheck, uuid string) bool {
	if check == nil {
		return true
	}
	if uuid == "" {
		return false
	}
	if check.CurrentUUID() != uuid {
		return false
	}

	timeout := kh.checkTimeoutDuration(check)
	if timeout <= 0 {
		return true
	}

	start := time.Unix(check.Status.LastRunUnix, 0)
	if start.IsZero() {
		return true
	}

	// block reports once the configured timeout has elapsed; the watcher will record the failure shortly after
	return time.Since(start) < timeout
}

// StartCheck begins tracking and managing a khcheck. This occurs when a khcheck is added.
func (kh *Kuberhealthy) StartCheck(khcheck *khapi.KuberhealthyCheck) error {
	log.Infoln("Starting Kuberhealthy check", khcheck.GetNamespace(), khcheck.GetName())

	// create a NamespacedName for additional calls
	checkName := types.NamespacedName{
		Namespace: khcheck.GetNamespace(),
		Name:      khcheck.GetName(),
	}

	// use CurrentUUID to signal the check is running
	if err := kh.setFreshUUID(checkName); err != nil {
		return fmt.Errorf("unable to set running UUID: %w", err)
	}

	// record the start time so we can persist it once the pod exists
	startTime := time.Now()
	khcheck.Status.LastRunUnix = startTime.Unix()

	// craft a full pod spec using the check's pod spec
	podSpec := kh.CheckPodSpec(khcheck)

	// create the checker pod and unwind the run metadata if scheduling fails
	if err := kh.CheckClient.Create(kh.Context, podSpec); err != nil {
		if kh.Recorder != nil {
			kh.Recorder.Event(khcheck, corev1.EventTypeWarning, "PodCreateFailed", fmt.Sprintf("failed to create pod: %v", err))
		}
		creationErr := fmt.Errorf("failed to create check pod: %w", err)
		if statusErr := kh.recordPodCreationFailure(checkName, creationErr); statusErr != nil {
			log.Errorf("start check: failed to persist pod creation failure for %s/%s: %v", khcheck.Namespace, khcheck.Name, statusErr)
		}
		if clearErr := kh.clearUUID(checkName); clearErr != nil {
			log.Errorf("start check: failed to clear uuid for %s/%s after pod creation error: %v", khcheck.Namespace, khcheck.Name, clearErr)
		}
		return creationErr
	}

	// persist the start time now that the pod exists so timeout tracking resumes accurately
	if err := kh.setLastRunTime(checkName, startTime); err != nil {
		return fmt.Errorf("unable to set check start time: %w", err)
	}
	if kh.Recorder != nil {
		kh.Recorder.Eventf(khcheck, corev1.EventTypeNormal, "PodStarted", "check pod scheduled at %s", startTime.Format(time.RFC3339))
	}
	log.WithFields(log.Fields{
		"namespace": khcheck.Namespace,
		"name":      khcheck.Name,
		"pod":       podSpec.Name,
	}).Info("Created checker pod")
	if kh.Recorder != nil {
		kh.Recorder.Eventf(khcheck, corev1.EventTypeNormal, "PodCreated", "created pod %s", podSpec.Name)
	}

	freshCheck, err := kh.readCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to refresh check %s/%s after start: %w", khcheck.Namespace, khcheck.Name, err)
	}
	kh.startTimeoutWatcher(freshCheck)
	return nil
}

// CheckPodSpec builds a pod for this check's run.
func (kh *Kuberhealthy) CheckPodSpec(khcheck *khapi.KuberhealthyCheck) *corev1.Pod {

	// generate a random suffix and concatenate a unique pod name
	suffix := uuid.NewString()
	if len(suffix) > 5 {
		suffix = suffix[:5]
	}
	podName := fmt.Sprintf("%s-%s", khcheck.GetName(), suffix)

	checkName := types.NamespacedName{Namespace: khcheck.Namespace, Name: khcheck.Name}
	uuid, err := kh.getCurrentUUID(checkName)
	if err != nil {
		log.Errorf("failed to get check uuid: %v", err)
	}

	// formulate a full pod spec
	podSpec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        podName,
			Namespace:   khcheck.GetNamespace(),
			Annotations: map[string]string{},
			Labels:      map[string]string{},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(khcheck, khapi.GroupVersion.WithKind("KuberhealthyCheck")),
			},
		},
		Spec: khcheck.Spec.PodSpec.Spec,
	}

	if md := khcheck.Spec.PodSpec.Metadata; md != nil {
		for k, v := range md.Annotations {
			podSpec.Annotations[k] = v
		}
		for k, v := range md.Labels {
			podSpec.Labels[k] = v
		}
	}
	for k, v := range khcheck.Spec.ExtraAnnotations {
		podSpec.Annotations[k] = v
	}
	for k, v := range khcheck.Spec.ExtraLabels {
		podSpec.Labels[k] = v
	}

	// add required annotations
	podSpec.Annotations["createdBy"] = "kuberhealthy"
	podSpec.Annotations[runUUIDLabel] = uuid
	// reference the check's last run time instead of the pod spec's creation timestamp
	podSpec.Annotations["createdTime"] = time.Unix(khcheck.Status.LastRunUnix, 0).String()
	podSpec.Annotations["kuberhealthyCheckName"] = khcheck.Name

	// add required labels
	podSpec.Labels[checkLabel] = khcheck.Name
	podSpec.Labels[runUUIDLabel] = uuid

	envVars := []corev1.EnvVar{{Name: envs.KHReportingURL, Value: kh.ReportingURL}, {Name: envs.KHRunUUID, Value: uuid}}
	for i := range podSpec.Spec.Containers {
		c := &podSpec.Spec.Containers[i]
		for _, v := range envVars {
			c.Env = setEnvVar(c.Env, v)
		}
	}
	for i := range podSpec.Spec.InitContainers {
		c := &podSpec.Spec.InitContainers[i]
		for _, v := range envVars {
			c.Env = setEnvVar(c.Env, v)
		}
	}

	return podSpec
}

func setEnvVar(vars []corev1.EnvVar, v corev1.EnvVar) []corev1.EnvVar {
	for i := range vars {
		if vars[i].Name == v.Name {
			vars[i] = v
			return vars
		}
	}
	return append(vars, v)
}

// StartCheck stops tracking and managing a khcheck. This occurs when a khcheck is removed.
func (kh *Kuberhealthy) StopCheck(khcheck *khapi.KuberhealthyCheck) error {
	log.Infoln("Stopping Kuberhealthy check", khcheck.GetNamespace(), khcheck.GetName())

	// clear CurrentUUID to indicate the check is no longer running
	oldUUID := khcheck.CurrentUUID()
	checkName := createNamespacedName(khcheck.GetName(), khcheck.GetNamespace())
	kh.clearUUID(checkName)

	// calculate the run time and record it
	lastRunTime, err := kh.getLastRunTime(checkName)
	if err != nil {
		return err
	}

	// calculate the last run duration and store it
	runTime := time.Since(lastRunTime)
	err = kh.setRunDuration(checkName, runTime)
	if err != nil {
		return err
	}

	if oldUUID == "" {
		return nil
	}

	var podList corev1.PodList
	err = kh.CheckClient.List(kh.Context, &podList,
		client.InNamespace(khcheck.GetNamespace()),
		client.MatchingLabels(map[string]string{runUUIDLabel: oldUUID}),
	)
	if err != nil {
		return fmt.Errorf("failed to list check pods: %w", err)
	}
	for i := range podList.Items {
		podRef := &podList.Items[i]
		if err := kh.CheckClient.Delete(kh.Context, podRef); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete checker pod %s: %w", podRef.Name, err)
		}
		log.WithFields(log.Fields{
			"namespace": khcheck.Namespace,
			"name":      khcheck.Name,
			"pod":       podRef.Name,
		}).Info("Deleted checker pod")
	}
	return nil
}

// UpdateCheck handles the event of a check getting updated in place
func (kh *Kuberhealthy) UpdateCheck(oldKHCheck *khapi.KuberhealthyCheck, newKHCheck *khapi.KuberhealthyCheck) error {
	log.WithFields(log.Fields{
		"namespace": newKHCheck.Namespace,
		"name":      newKHCheck.Name,
		"pod":       newKHCheck.Name,
	}).Info("KHCheck resoruce updated")
	// TODO - do we do anything on updates to reload the latest check? How do we prevent locking into an infinite udpate window with the controller?

	// // stop the check
	// err := kh.StopCheck(oldKHCheck)
	// if err != nil {
	// 	return fmt.Errorf("failed to stop check for updating: %w", err)
	// }

	// // start the check again
	// err = kh.StartCheck(newKHCheck)
	// if err != nil {
	// 	return fmt.Errorf("failed to start check after updating: %w", err)
	// }
	return nil
}

// IsStarted returns if this instance is running or not
func (kh *Kuberhealthy) IsStarted() bool {
	return kh.running
}

// runReaper periodically scans all khcheck pods and cleans up any that have exceeded their configured runtime or lingered after completion.
func (kh *Kuberhealthy) runReaper(ctx context.Context, interval time.Duration) {
	// tick on the requested cadence so we sweep for pods that can be removed from the cluster
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// controller is shutting down so there is no more cleanup to perform
			return
		case <-ticker.C:
			// sweep once per tick and log failures so operators can diagnose cleanups that stall
			err := kh.reapOnce()
			if err != nil {
				log.Errorln("reaper:", err)
			}
		}
	}
}

// reapOnce performs a single scan of khchecks and applies cleanup logic. It is primarily exposed for unit testing.
func (kh *Kuberhealthy) reapOnce() error {
	// gather every khcheck so we can inspect their pod history
	var checkList khapi.KuberhealthyCheckList
	err := kh.CheckClient.List(kh.Context, &checkList)
	if err != nil {
		return err
	}
	for i := range checkList.Items {
		// EnsureCreationTimestamp backfills metadata for fake clients in unit tests
		checkList.Items[i].EnsureCreationTimestamp()
	}

	// figure out pod retention limits from the environment with sane defaults
	retention := time.Duration(defaultFailedPodRetentionDays) * 24 * time.Hour
	value := os.Getenv("KH_ERROR_POD_RETENTION_DAYS")
	if value != "" {
		days, parseErr := strconv.Atoi(value)
		if parseErr == nil && days > 0 {
			retention = time.Duration(days) * 24 * time.Hour
		}
	}

	maxFailed := defaultMaxFailedPods
	value = os.Getenv("KH_MAX_ERROR_POD_COUNT")
	if value != "" {
		count, parseErr := strconv.Atoi(value)
		if parseErr == nil && count > 0 {
			maxFailed = count
		}
	}

	for i := range checkList.Items {
		check := &checkList.Items[i]

		// start from default scheduling values so all math has a baseline
		timeout := defaultRunTimeout
		runInterval := defaultRunInterval

		raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(check)
		if err == nil {
			spec, ok := raw["spec"].(map[string]interface{})
			if ok {
				if v, found := spec["timeout"].(string); found {
					d, parseErr := time.ParseDuration(v)
					if parseErr == nil {
						timeout = d
					}
				}
				if v, found := spec["runInterval"].(string); found {
					d, parseErr := time.ParseDuration(v)
					if parseErr == nil {
						runInterval = d
					}
				}
			}
		}

		var podList corev1.PodList
		err = kh.CheckClient.List(kh.Context, &podList,
			client.InNamespace(check.Namespace),
			client.HasLabels{runUUIDLabel},
		)
		if err != nil {
			log.Errorf("reaper: list pods for %s/%s: %v", check.Namespace, check.Name, err)
			continue
		}

		// keep pods that already failed separate so we can apply retention after the main sweep
		var failedPods []corev1.Pod

		runStart := time.Unix(check.Status.LastRunUnix, 0)
		// compute how long the current run has been active so we can compare against deadlines
		var runAge time.Duration
		if !runStart.IsZero() {
			runAge = time.Since(runStart)
		}

		for pod := range podList.Items {
			podRef := &podList.Items[pod]
			if podRef.Labels[checkLabel] != check.Name {
				// a stale pod from another check slipped through the label selector
				continue
			}

			switch podRef.Status.Phase {
			case corev1.PodRunning, corev1.PodPending, corev1.PodUnknown:
				// the checker is still running so enforce the timeout floor before deleting it
				maxAge := timeout
				if maxAge < 5*time.Minute {
					// the business requirement guarantees at least a five minute window before reaping
					maxAge = 5 * time.Minute
				}

				if runAge > maxAge {
					// the check already timed out and survived the grace period so remove the pod
					err = kh.CheckClient.Delete(kh.Context, podRef)
					if err != nil && !apierrors.IsNotFound(err) {
						log.Errorf("reaper: failed deleting timed out pod %s/%s: %v", podRef.Namespace, podRef.Name, err)
						continue
					}

					log.WithFields(log.Fields{
						"namespace": check.Namespace,
						"name":      check.Name,
						"pod":       podRef.Name,
					}).Info("deleted checker pod")

					if kh.Recorder != nil {
						kh.Recorder.Eventf(check, corev1.EventTypeWarning, "CheckRunTimeout", "deleted pod %s after exceeding timeout %s", podRef.Name, timeout)
					}
				}
			case corev1.PodSucceeded:
				// completed pods stick around for a while so operators can inspect their logs
				if runAge > runInterval*10 {
					err = kh.CheckClient.Delete(kh.Context, podRef)
					if err != nil && !apierrors.IsNotFound(err) {
						log.Errorf("reaper: failed deleting completed pod %s/%s: %v", podRef.Namespace, podRef.Name, err)
						continue
					}

					log.WithFields(log.Fields{
						"namespace": check.Namespace,
						"name":      check.Name,
						"pod":       podRef.Name,
					}).Info("deleted checker pod")

					if kh.Recorder != nil {
						kh.Recorder.Eventf(check, corev1.EventTypeNormal, "CheckPodReaped", "deleted completed pod %s after %s", podRef.Name, runAge)
					}
				}
			case corev1.PodFailed:
				// failed pods are evaluated after the main loop so we can respect both count and age limits
				failedPods = append(failedPods, *podRef)
			default:
				// logging unknown phases keeps us aware of new Kubernetes pod states
				log.Errorf("reaper: encountered pod %s/%s with unexpected phase %s", podRef.Namespace, podRef.Name, podRef.Status.Phase)
			}
		}

		// delete the oldest failed pods first so we retain recent failure data for debugging
		sort.Slice(failedPods, func(i, j int) bool {
			return failedPods[i].Name > failedPods[j].Name
		})

		for pod := range failedPods {
			podRef := &failedPods[pod]
			if pod >= maxFailed || runAge > retention {
				// either the failure backlog exceeds our quota or the pod aged past retention, so delete it
				err = kh.CheckClient.Delete(kh.Context, podRef)
				if err != nil && !apierrors.IsNotFound(err) {
					log.Errorf("reaper: failed deleting failed pod %s/%s: %v", podRef.Namespace, podRef.Name, err)
					continue
				}

				log.WithFields(log.Fields{
					"namespace": check.Namespace,
					"name":      check.Name,
					"pod":       podRef.Name,
				}).Info("deleted checker pod")

				if kh.Recorder != nil {
					kh.Recorder.Eventf(check, corev1.EventTypeNormal, "CheckFailedPodReaped", "removed failed pod %s after %s", podRef.Name, runAge)
				}
			}
		}
	}

	return nil
}

// readCheck fetches a check from the cluster.
func (k *Kuberhealthy) readCheck(checkName types.NamespacedName) (*khapi.KuberhealthyCheck, error) {
	return khapi.GetCheck(k.Context, k.CheckClient, checkName)
}

// setCheckExecutionError sets an execution error for a khcheck in its crd status
func (k *Kuberhealthy) setCheckExecutionError(checkName types.NamespacedName, checkErrors []string) error {

	// get the check as it is right now
	khCheck, err := k.readCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	// set the errors
	khCheck.Status.Errors = checkErrors
	if len(checkErrors) > 0 {
		khCheck.Status.ConsecutiveFailures++
	}

	// update the khcheck resource
	err = k.CheckClient.Status().Update(k.Context, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update check errors with error: %w", err)
	}
	return nil
}

// // setUUID sets the specified UUID on the check
// func (k *Kuberhealthy) setUUID(checkName types.NamespacedName, uuid string) error {

// 	// get the check as it is right now
// 	khCheck, err := k.getCheck(checkName)
// 	if err != nil {
// 		return fmt.Errorf("failed to get check: %w", err)
// 	}

// 	// set the errors
// 	khCheck.Status.CurrentUUID = uuid

// 	// update the khcheck resource
// 	err = k.CheckClient.Status().Update(k.Context, khCheck)
// 	if err != nil {
// 		return fmt.Errorf("failed to update check uuid with error: %w", err)
// 	}
// 	return nil
// }

// clearUUID clears the UUID assigned to the check, which indicates
// that it is not running.
func (k *Kuberhealthy) clearUUID(checkName types.NamespacedName) error {

	// get the check as it is right now
	khCheck, err := k.readCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	// set the errors
	khCheck.SetCurrentUUID("")

	// update the khcheck resource
	err = khapi.UpdateCheck(k.Context, k.CheckClient, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update check with fresh uuid with error: %w", err)
	}
	return nil
}

// setFreshUUID generates a UUID and sets it on the check.
func (k *Kuberhealthy) setFreshUUID(checkName types.NamespacedName) error {

	// get the check as it is right now
	khCheck, err := k.readCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	// set the errors
	khCheck.SetCurrentUUID(uuid.NewString())

	// update the khcheck resource
	err = khapi.UpdateCheck(k.Context, k.CheckClient, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update check with fresh uuid with error: %w", err)
	}
	return nil
}

// setOK sets the OK property on the status of a khcheck
func (k *Kuberhealthy) setOK(checkName types.NamespacedName, ok bool) error {
	khCheck, err := k.readCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	khCheck.Status.OK = ok
	if ok {
		khCheck.Status.ConsecutiveFailures = 0
	}

	err = k.CheckClient.Status().Update(k.Context, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update OK status: %w", err)
	}

	return nil
}

// getLastRunTime gets the last run start time unix time
func (k *Kuberhealthy) getLastRunTime(checkName types.NamespacedName) (time.Time, error) {
	khCheck, err := k.readCheck(checkName)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get check: %w", err)
	}

	// secs is your Unix timestamp in seconds (int64)
	t := time.Unix(khCheck.Status.LastRunUnix, 0).UTC()
	return t, nil
}

// setLastRunTime sets the last run start time unix time
func (k *Kuberhealthy) setLastRunTime(checkName types.NamespacedName, lastRunTime time.Time) error {
	khCheck, err := k.readCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	khCheck.Status.LastRunUnix = lastRunTime.Unix()

	err = khapi.UpdateCheck(k.Context, k.CheckClient, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update LastRun time: %w", err)
	}

	return nil
}

// setRunDuration sets the time the last check took to run
func (k *Kuberhealthy) setRunDuration(checkName types.NamespacedName, runDuration time.Duration) error {
	khCheck, err := k.readCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	khCheck.Status.LastRunDuration = runDuration

	err = khapi.UpdateCheck(k.Context, k.CheckClient, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update RunDuration: %w", err)
	}

	return nil
}

// isCheckRunning returns true if the check's CurrentUUID is set because we assume
// checks only have a CurrentUUID when they are running.
func (k *Kuberhealthy) isCheckRunning(checkName types.NamespacedName) (bool, error) {
	uuid, err := k.getCurrentUUID(checkName)
	if err != nil {
		return false, err
	}
	return uuid != "", nil
}

// getCurrentUUID returns the CurrentUUID string for the specified check.
func (k *Kuberhealthy) getCurrentUUID(checkName types.NamespacedName) (string, error) {
	khCheck, err := k.readCheck(checkName)
	if err != nil {
		return "", fmt.Errorf("failed to get check: %w", err)
	}

	return khCheck.CurrentUUID(), nil
}
