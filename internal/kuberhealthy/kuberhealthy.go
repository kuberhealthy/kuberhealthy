package kuberhealthy

import (
	"context"
	"fmt"
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
	"k8s.io/client-go/kubernetes"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	record "k8s.io/client-go/tools/record"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	restconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// defaultRunInterval is used when a check does not specify a runInterval or it fails to parse.
	defaultRunInterval = time.Minute * 10
	// scheduleLoopInterval controls how often Kuberhealthy scans for checks to run.
	scheduleLoopInterval = 30 * time.Second
	checkLabel           = "khcheck"
	runUUIDLabel         = "kh-run-uuid"
)

// Kuberhealthy handles background processing for checks
type Kuberhealthy struct {
	Context     context.Context
	cancel      context.CancelFunc
	running     bool          // indicates that Start() has been called and this instance is running
	CheckClient client.Client // Kubernetes client for check CRUD

	Recorder record.EventRecorder // emits k8s events for khcheck lifecycle

	ReportingURL string

	loopMu      sync.Mutex
	loopRunning bool
}

// New creates a new Kuberhealthy instance and event recorder
func New(ctx context.Context, checkClient client.Client) *Kuberhealthy {
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

	return &Kuberhealthy{
		Context:     ctx,
		CheckClient: checkClient,
		Recorder:    recorder,
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
	go kh.startScheduleLoop()
	go kh.runReaper(kh.Context, time.Minute)
	if cfg != nil {
		go kh.startKHCheckWatch(cfg)
	}

	log.Infoln("Kuberhealthy start")
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
		// log metadata on the pod spec to debug unexpected fields
		debugKHCheckMetadata(&check)

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

	// record the start time
	startTime := time.Now()
	if err := kh.setLastRunTime(checkName, startTime); err != nil {
		return fmt.Errorf("unable to set check start time: %w", err)
	}
	khcheck.Status.LastRunUnix = startTime.Unix()
	if kh.Recorder != nil {
		kh.Recorder.Eventf(khcheck, corev1.EventTypeNormal, "PodStarted", "check pod scheduled at %s", startTime.Format(time.RFC3339))
	}

	// craft a full pod spec using the check's pod spec
	podSpec := kh.CheckPodSpec(khcheck)

	// create the checker pod
	if err := kh.CheckClient.Create(kh.Context, podSpec); err != nil {
		if kh.Recorder != nil {
			kh.Recorder.Event(khcheck, corev1.EventTypeWarning, "PodCreateFailed", fmt.Sprintf("failed to create pod: %v", err))
		}
		return fmt.Errorf("failed to create check pod: %w", err)
	}
	log.WithFields(log.Fields{
		"namespace": khcheck.Namespace,
		"name":      khcheck.Name,
		"pod":       podSpec.Name,
	}).Info("created checker pod")
	if kh.Recorder != nil {
		kh.Recorder.Eventf(khcheck, corev1.EventTypeNormal, "PodCreated", "created pod %s", podSpec.Name)
	}
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
		}).Info("deleted checker pod")
	}
	return nil
}

// UpdateCheck handles the event of a check getting updated in place
func (kh *Kuberhealthy) UpdateCheck(oldKHCheck *khapi.KuberhealthyCheck, newKHCheck *khapi.KuberhealthyCheck) error {
	log.Infoln("Updating Kuberhealthy check", oldKHCheck.GetNamespace(), oldKHCheck.GetName())
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

// debugKHCheckMetadata logs pod spec metadata when present
func debugKHCheckMetadata(khCheck *khapi.KuberhealthyCheck) {
	if khCheck == nil {
		return
	}
	log.WithFields(log.Fields{
		"namespace": khCheck.Namespace,
		"name":      khCheck.Name,
		"metadata":  khCheck.GetObjectMeta(),
	}).Debug("khcheck podSpec metadata")
}

// readCheck fetches a check from the cluster.
func (k *Kuberhealthy) readCheck(checkName types.NamespacedName) (*khapi.KuberhealthyCheck, error) {
	khCheck, err := khapi.GetCheck(k.Context, k.CheckClient, checkName)
	if err == nil {
		debugKHCheckMetadata(khCheck)
	}
	return khCheck, err
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
