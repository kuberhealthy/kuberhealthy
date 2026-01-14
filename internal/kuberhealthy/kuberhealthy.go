package kuberhealthy

import (
	"context"
	"fmt"
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
	// minimumScheduleInterval controls the shortest delay between scheduling scans when no check is due immediately.
	minimumScheduleInterval = time.Second
	// primaryCheckLabel is the canonical label key applied to checker pods so other components can find them quickly.
	primaryCheckLabel = "healthcheck"
	runUUIDLabel      = "kh-run-uuid"
	// timeoutGracePeriod adds a tiny buffer before flagging a run as failed so pods can finish cleanly at the
	// deadline without tripping a race.
	timeoutGracePeriod = 2 * time.Second
	// defaultMaxFailedPods is the maximum number of failed pods to retain for a check.
	defaultMaxFailedPods = 3
)

var (
	healthCheckGVR     = khapi.GroupVersion.WithResource("healthchecks")
	recorderSchemeOnce sync.Once
	recorderSchemeErr  error
)

// Kuberhealthy handles background processing for checks
type Kuberhealthy struct {
	Context     context.Context
	cancel      context.CancelFunc
	running     bool          // indicates that Start() has been called and this instance is running
	CheckClient client.Client // Kubernetes client for check CRUD
	restConfig  *rest.Config  // cached config for building informers once start() wires everything together

	Recorder record.EventRecorder // emits Kubernetes events for HealthCheck lifecycle transitions

	ReportingURL string

	loopMu      sync.Mutex
	loopRunning bool
	doneChan    chan struct{} // signaled when shutdown completes

}

// New creates a new Kuberhealthy instance, event recorder, and optional shutdown notifier.
// The shutdown channel can be omitted or passed as nil if not needed.
func New(ctx context.Context, checkClient client.Client, doneChan ...chan struct{}) *Kuberhealthy {
	log.Infoln("New Kuberhealthy instance created")

	// build an event recorder when the environment supports it
	recorder := newEventRecorder()

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

// newEventRecorder configures a Kubernetes event recorder for surfacing lifecycle events.
func newEventRecorder() record.EventRecorder {
	// load the controller configuration so we can talk to the API server
	cfg, err := restconfig.GetConfig()
	if err != nil {
		log.Errorln("event recorder disabled:", err)
		return nil
	}

	// create a clientset for emitting core/v1 events
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Errorln("event recorder disabled:", err)
		return nil
	}

	// broadcast events to the cluster so operators see lifecycle transitions
	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: cs.CoreV1().Events("")})

	recorderSchemeOnce.Do(func() {
		recorderSchemeErr = khapi.AddToScheme(k8scheme.Scheme)
	})
	if recorderSchemeErr != nil {
		log.Errorln("event recorder disabled:", recorderSchemeErr)
		return nil
	}

	// create a recorder scoped to the kuberhealthy component name
	return broadcaster.NewRecorder(k8scheme.Scheme, corev1.EventSource{Component: "kuberhealthy"})
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

// Start begins background processing for HealthCheck resources.
// A Kubernetes rest.Config is optional; when provided, healthchecks will be
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

// startScheduleLoop periodically evaluates all known HealthChecks and starts new runs when due.
func (kh *Kuberhealthy) startScheduleLoop() {
	if !kh.setLoopRunning(true) {
		return
	}
	defer kh.setLoopRunning(false)

	delay := kh.scheduleChecks()
	if delay <= 0 {
		delay = minimumScheduleInterval
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	for {
		select {
		case <-kh.Context.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
			delay = kh.scheduleChecks()
			if delay <= 0 {
				delay = minimumScheduleInterval
			}
			timer.Reset(delay)
		}
	}
}

// scheduleChecks iterates through all HealthChecks, starts any that are due to run, and returns the shortest delay before the next evaluation should occur.
func (kh *Kuberhealthy) scheduleChecks() time.Duration {
	uList := &unstructured.UnstructuredList{}
	uList.SetGroupVersionKind(khapi.GroupVersion.WithKind("HealthCheckList"))
	err := kh.CheckClient.List(kh.Context, uList)
	if err != nil {
		log.Errorln("failed to list healthchecks:", err)
		return minimumScheduleInterval
	}

	nextDelay := time.Duration(0)
	started := false
	now := time.Now()
	for _, healthCheckItem := range uList.Items {
		runInterval := kh.runIntervalForCheck(&healthCheckItem)

		var check khapi.HealthCheck
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(healthCheckItem.Object, &check)
		if err != nil {
			log.Errorf("failed to convert check %s/%s: %v", healthCheckItem.GetNamespace(), healthCheckItem.GetName(), err)
			continue
		}
		check.EnsureCreationTimestamp()

		lastStart := time.Unix(check.Status.LastRunUnix, 0)

		// determine when this check should run next
		remaining := runInterval - now.Sub(lastStart)
		if lastStart.IsZero() {
			remaining = 0
		}

		if check.CurrentUUID() != "" {
			nextDelay = minPositiveDuration(nextDelay, remaining)
			continue
		}

		if remaining > 0 {
			nextDelay = minPositiveDuration(nextDelay, remaining)
			continue
		}

		err = kh.StartCheck(&check)
		if err != nil {
			log.Errorf("failed to start check %s/%s: %v", check.Namespace, check.Name, err)
			continue
		}
		started = true
		nextDelay = minPositiveDuration(nextDelay, runInterval)
	}

	if started {
		return minimumScheduleInterval
	}
	if nextDelay <= 0 {
		return minimumScheduleInterval
	}
	if nextDelay < minimumScheduleInterval {
		return minimumScheduleInterval
	}
	return nextDelay
}

// minPositiveDuration returns the smallest positive duration, ignoring non-positive candidates.
func minPositiveDuration(current, candidate time.Duration) time.Duration {
	if candidate <= 0 {
		return current
	}
	if current <= 0 || candidate < current {
		return candidate
	}
	return current
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

// startKHCheckWatch begins watching HealthCheck resources and reacts to events.
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
	inf := factory.ForResource(healthCheckGVR).Informer()

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

// handleCreate registers a finalizer on new checks and starts their initial run.
func (kh *Kuberhealthy) handleCreate(khc *khapi.HealthCheck) {
	err := kh.addFinalizer(kh.Context, khc)
	if err != nil {
		log.Errorln("error:", err)
		return
	}

	err = kh.StartCheck(khc)
	if err != nil {
		log.Errorln("error:", err)
	}
}

// handleUpdate reconciles finalizers and restarts checks when their spec changes.
func (kh *Kuberhealthy) handleUpdate(oldCheck *khapi.HealthCheck, newCheck *khapi.HealthCheck) {
	if !newCheck.GetDeletionTimestamp().IsZero() {
		if kh.hasFinalizer(newCheck) {
			err := kh.StopCheck(newCheck)
			if err != nil {
				log.Errorln("error:", err)
			}

			nn := types.NamespacedName{Namespace: newCheck.Namespace, Name: newCheck.Name}
			refreshed, err := khapi.GetCheck(kh.Context, kh.CheckClient, nn)
			if err != nil {
				log.Errorln("error:", err)
				return
			}

			err = kh.deleteFinalizer(kh.Context, refreshed)
			if err != nil {
				log.Errorln("error:", err)
			}
		}
		return
	}

	log.WithFields(log.Fields{
		"namespace": newCheck.Namespace,
		"name":      newCheck.Name,
	}).Info("modified checker pod")

	err := kh.UpdateCheck(oldCheck, newCheck)
	if err != nil {
		log.Errorln("error:", err)
	}
}

// handleDelete removes finalizers and ensures pods are terminated when a check disappears.
func (kh *Kuberhealthy) handleDelete(khc *khapi.HealthCheck) {
	if kh.hasFinalizer(khc) {
		err := kh.StopCheck(khc)
		if err != nil {
			log.Errorln("error:", err)
		}

		nn := types.NamespacedName{Namespace: khc.Namespace, Name: khc.Name}
		refreshed, err := khapi.GetCheck(kh.Context, kh.CheckClient, nn)
		if err != nil {
			log.Errorln("error:", err)
			return
		}

		err = kh.deleteFinalizer(kh.Context, refreshed)
		if err != nil {
			log.Errorln("error:", err)
		}
		return
	}

	err := kh.StopCheck(khc)
	if err != nil {
		log.Errorln("error:", err)
	}
}

// resumeCheckTimeouts ensures active checks pick up timeout monitoring when Kuberhealthy restarts so that
// previously scheduled runs still fail at the correct time.
func (kh *Kuberhealthy) resumeCheckTimeouts() error {
	if kh.CheckClient == nil {
		return fmt.Errorf("check client not configured")
	}

	// pull every HealthCheck so we can resume timers for anything currently marked as running
	list := &khapi.HealthCheckList{}
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
func (kh *Kuberhealthy) startTimeoutWatcher(check *khapi.HealthCheck) {
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

// checkTimeoutDuration returns the runtime limit for the provided check, defaulting when unset.
func (kh *Kuberhealthy) checkTimeoutDuration(check *khapi.HealthCheck) time.Duration {
	if check == nil {
		return 0
	}
	if check.Spec.Timeout != nil && check.Spec.Timeout.Duration > 0 {
		return check.Spec.Timeout.Duration
	}
	return defaultRunTimeout
}

// IsReportAllowed returns true when a checker pod is still permitted to report its result via the /check endpoint.
func (kh *Kuberhealthy) IsReportAllowed(check *khapi.HealthCheck, uuid string) bool {
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

// StartCheck begins tracking and managing a HealthCheck whenever the controller observes a new resource.
func (kh *Kuberhealthy) StartCheck(healthCheck *khapi.HealthCheck) error {
	log.Infoln("Starting healthcheck", healthCheck.GetNamespace(), healthCheck.GetName())

	// create a NamespacedName for additional calls
	checkName := types.NamespacedName{
		Namespace: healthCheck.GetNamespace(),
		Name:      healthCheck.GetName(),
	}

	// use CurrentUUID to signal the check is running
	err := kh.setFreshUUID(checkName)
	if err != nil {
		return fmt.Errorf("unable to set running UUID: %w", err)
	}

	// record the start time so we can persist it once the pod exists
	startTime := time.Now()
	healthCheck.Status.LastRunUnix = startTime.Unix()

	// craft a full pod spec using the check's pod spec
	podSpec := kh.CheckPodSpec(healthCheck)

	// create the checker pod and unwind the run metadata if scheduling fails
	err = kh.CheckClient.Create(kh.Context, podSpec)
	if err != nil {
		if kh.Recorder != nil {
			kh.Recorder.Event(healthCheck, corev1.EventTypeWarning, "PodCreateFailed", fmt.Sprintf("failed to create pod: %v", err))
		}
		creationErr := fmt.Errorf("failed to create check pod: %w", err)
		statusErr := kh.recordPodCreationFailure(checkName, creationErr)
		if statusErr != nil {
			log.Errorf("start check: failed to persist pod creation failure for %s/%s: %v", healthCheck.Namespace, healthCheck.Name, statusErr)
		}
		clearErr := kh.clearUUID(checkName)
		if clearErr != nil {
			log.Errorf("start check: failed to clear uuid for %s/%s after pod creation error: %v", healthCheck.Namespace, healthCheck.Name, clearErr)
		}
		return creationErr
	}

	// persist the start time now that the pod exists so timeout tracking resumes accurately
	err = kh.setLastRunTime(checkName, startTime)
	if err != nil {
		return fmt.Errorf("unable to set check start time: %w", err)
	}
	if kh.Recorder != nil {
		kh.Recorder.Eventf(healthCheck, corev1.EventTypeNormal, "PodStarted", "check pod scheduled at %s", startTime.Format(time.RFC3339))
	}
	log.WithFields(log.Fields{
		"namespace": healthCheck.Namespace,
		"name":      healthCheck.Name,
		"pod":       podSpec.Name,
	}).Info("Created checker pod")
	if kh.Recorder != nil {
		kh.Recorder.Eventf(healthCheck, corev1.EventTypeNormal, "PodCreated", "created pod %s", podSpec.Name)
	}

	freshCheck, err := kh.readCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to refresh check %s after start: %w", checkName.String(), err)
	}
	kh.startTimeoutWatcher(freshCheck)
	return nil
}

// CheckPodSpec builds a pod for this check's run.
func (kh *Kuberhealthy) CheckPodSpec(healthCheck *khapi.HealthCheck) *corev1.Pod {

	// generate a random suffix and concatenate a unique pod name
	suffix := uuid.NewString()
	if len(suffix) > 5 {
		suffix = suffix[:5]
	}
	podName := fmt.Sprintf("%s-%s", healthCheck.GetName(), suffix)

	checkName := types.NamespacedName{Namespace: healthCheck.Namespace, Name: healthCheck.Name}
	uuid, err := kh.getCurrentUUID(checkName)
	if err != nil {
		log.Errorf("failed to get check uuid: %v", err)
	}

	// formulate a full pod spec
	podSpec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        podName,
			Namespace:   healthCheck.GetNamespace(),
			Annotations: map[string]string{},
			Labels:      map[string]string{},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(healthCheck, khapi.GroupVersion.WithKind("HealthCheck")),
			},
		},
		Spec: healthCheck.Spec.PodSpec.Spec,
	}

	if md := healthCheck.Spec.PodSpec.Metadata; md != nil {
		for k, v := range md.Annotations {
			podSpec.Annotations[k] = v
		}
		for k, v := range md.Labels {
			podSpec.Labels[k] = v
		}
	}
	for k, v := range healthCheck.Spec.ExtraAnnotations {
		podSpec.Annotations[k] = v
	}
	for k, v := range healthCheck.Spec.ExtraLabels {
		podSpec.Labels[k] = v
	}

	// add required annotations
	podSpec.Annotations["createdBy"] = "kuberhealthy"
	podSpec.Annotations[runUUIDLabel] = uuid
	// reference the check's last run time instead of the pod spec's creation timestamp
	podSpec.Annotations["createdTime"] = time.Unix(healthCheck.Status.LastRunUnix, 0).String()
	podSpec.Annotations["kuberhealthyCheckName"] = healthCheck.Name

	// add required labels
	podSpec.Labels[primaryCheckLabel] = healthCheck.Name
	podSpec.Labels[runUUIDLabel] = uuid

	envVars := []corev1.EnvVar{{Name: envs.KHReportingURL, Value: kh.ReportingURL}, {Name: envs.KHRunUUID, Value: uuid}}
	// Inject a run deadline so checks can self-timeout before the controller does.
	deadline, hasDeadline := kh.checkRunDeadlineUnix(healthCheck)
	if hasDeadline {
		envVars = append(envVars, corev1.EnvVar{Name: envs.KHDeadline, Value: deadline})
	}
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

// checkRunDeadlineUnix returns the run deadline as a unix timestamp string when available.
func (kh *Kuberhealthy) checkRunDeadlineUnix(check *khapi.HealthCheck) (string, bool) {
	if check == nil {
		return "", false
	}

	startedAt := time.Unix(check.Status.LastRunUnix, 0)
	if startedAt.IsZero() {
		return "", false
	}

	timeout := kh.checkTimeoutDuration(check)
	if timeout <= 0 {
		return "", false
	}

	deadline := startedAt.Add(timeout).Unix()
	return strconv.FormatInt(deadline, 10), true
}

// setEnvVar ensures the provided environment variable is present, replacing existing entries.
func setEnvVar(vars []corev1.EnvVar, v corev1.EnvVar) []corev1.EnvVar {
	for i := range vars {
		if vars[i].Name == v.Name {
			vars[i] = v
			return vars
		}
	}
	return append(vars, v)
}

// StopCheck stops tracking and managing a HealthCheck when the resource is removed.
func (kh *Kuberhealthy) StopCheck(healthCheck *khapi.HealthCheck) error {
	log.Infoln("Stopping healthcheck", healthCheck.GetNamespace(), healthCheck.GetName())

	// clear CurrentUUID to indicate the check is no longer running
	oldUUID := healthCheck.CurrentUUID()
	checkName := createNamespacedName(healthCheck.GetName(), healthCheck.GetNamespace())
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
		client.InNamespace(healthCheck.GetNamespace()),
		client.MatchingLabels(map[string]string{runUUIDLabel: oldUUID}),
	)
	if err != nil {
		return fmt.Errorf("failed to list check pods: %w", err)
	}
	for i := range podList.Items {
		podRef := &podList.Items[i]
		deleteErr := kh.CheckClient.Delete(kh.Context, podRef)
		if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
			return fmt.Errorf("failed to delete checker pod %s: %w", podRef.Name, deleteErr)
		}
		log.WithFields(log.Fields{
			"namespace": healthCheck.Namespace,
			"name":      healthCheck.Name,
			"pod":       podRef.Name,
		}).Info("Deleted checker pod")
	}
	return nil
}

// UpdateCheck handles the event of a check getting updated in place
func (kh *Kuberhealthy) UpdateCheck(oldKHCheck *khapi.HealthCheck, newKHCheck *khapi.HealthCheck) error {
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

// runReaper periodically scans all HealthCheck pods and cleans up any that have exceeded their configured runtime or lingered after completion.
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

// reapOnce performs a single scan of HealthChecks and applies cleanup logic. It is primarily exposed for unit testing.
func (kh *Kuberhealthy) reapOnce() error {
	// gather every HealthCheck so we can inspect their pod history
	var checkList khapi.HealthCheckList
	err := kh.CheckClient.List(kh.Context, &checkList)
	if err != nil {
		return err
	}
	for i := range checkList.Items {
		// EnsureCreationTimestamp backfills metadata for fake clients in unit tests
		checkList.Items[i].EnsureCreationTimestamp()
	}

	for i := range checkList.Items {
		check := &checkList.Items[i]
		cleanupErr := kh.cleanupPodsForCheck(check)
		if cleanupErr != nil {
			log.Errorf("reaper: cleanup pods for %s/%s: %v", check.Namespace, check.Name, cleanupErr)
		}
	}

	return nil
}

// cleanupPodsForCheck evaluates every pod owned by the provided check and deletes any that violate lifecycle policies.
func (kh *Kuberhealthy) cleanupPodsForCheck(check *khapi.HealthCheck) error {
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
		return fmt.Errorf("list pods: %w", err)
	}

	// compute how long the current run has been active so we can compare against deadlines
	runStart := time.Unix(check.Status.LastRunUnix, 0)
	var runAge time.Duration
	if !runStart.IsZero() {
		runAge = time.Since(runStart)
	}

	var failedPods []corev1.Pod

	for pod := range podList.Items {
		podRef := &podList.Items[pod]
		labelMatch := podRef.Labels[primaryCheckLabel] == check.Name
		if !labelMatch {
			// a stale pod from another check slipped through the label selector
			continue
		}

		switch podRef.Status.Phase {
		case corev1.PodRunning, corev1.PodPending, corev1.PodUnknown:
			maxAge := timeout * 2
			if maxAge < 5*time.Minute {
				maxAge = 5 * time.Minute
			}
			if runAge <= maxAge {
				continue
			}
			msg := fmt.Sprintf("deleted pod %s after exceeding timeout %s", podRef.Name, timeout)
			err := kh.deletePod(check, podRef, corev1.EventTypeWarning, "CheckRunTimeout", msg)
			if err != nil {
				log.Errorf("reaper: failed deleting timed out pod %s/%s: %v", podRef.Namespace, podRef.Name, err)
			}
		case corev1.PodSucceeded:
			if runAge <= runInterval*10 {
				continue
			}
			msg := fmt.Sprintf("deleted completed pod %s after %s", podRef.Name, runAge)
			err := kh.deletePod(check, podRef, corev1.EventTypeNormal, "CheckPodReaped", msg)
			if err != nil {
				log.Errorf("reaper: failed deleting completed pod %s/%s: %v", podRef.Namespace, podRef.Name, err)
			}
		case corev1.PodFailed:
			failedPods = append(failedPods, *podRef)
		default:
			log.Errorf("reaper: encountered pod %s/%s with unexpected phase %s", podRef.Namespace, podRef.Name, podRef.Status.Phase)
			failedPods = append(failedPods, *podRef)
		}
	}

	if len(failedPods) <= defaultMaxFailedPods {
		return nil
	}

	sort.Slice(failedPods, func(i, j int) bool {
		iTime := failedPods[i].CreationTimestamp.Time
		jTime := failedPods[j].CreationTimestamp.Time
		if !iTime.Equal(jTime) {
			return iTime.Before(jTime)
		}
		return failedPods[i].Name < failedPods[j].Name
	})

	for pod := 0; pod < len(failedPods)-defaultMaxFailedPods; pod++ {
		podRef := &failedPods[pod]
		msg := fmt.Sprintf("removed failed pod %s while trimming backlog", podRef.Name)
		err := kh.deletePod(check, podRef, corev1.EventTypeNormal, "CheckFailedPodReaped", msg)
		if err != nil {
			log.Errorf("reaper: failed deleting failed pod %s/%s: %v", podRef.Namespace, podRef.Name, err)
		}
	}

	return nil
}

// deletePod removes the given pod and emits the standard log and Kubernetes event entries.
func (kh *Kuberhealthy) deletePod(check *khapi.HealthCheck, pod *corev1.Pod, eventType, reason, message string) error {
	err := kh.CheckClient.Delete(kh.Context, pod)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	log.WithFields(log.Fields{
		"namespace": check.Namespace,
		"name":      check.Name,
		"pod":       pod.Name,
	}).Info("deleted checker pod")

	if kh.Recorder != nil {
		kh.Recorder.Eventf(check, eventType, reason, message)
	}

	return nil
}

// readCheck fetches a check from the cluster.
func (k *Kuberhealthy) readCheck(checkName types.NamespacedName) (*khapi.HealthCheck, error) {
	return khapi.GetCheck(k.Context, k.CheckClient, checkName)
}

// setCheckExecutionError sets an execution error for a HealthCheck in its CRD status
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

	// update the HealthCheck resource
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

// 	// update the HealthCheck resource
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

	// update the HealthCheck resource
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

	// update the HealthCheck resource
	err = khapi.UpdateCheck(k.Context, k.CheckClient, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update check with fresh uuid with error: %w", err)
	}
	return nil
}

// setOK sets the OK property on the status of a HealthCheck
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
