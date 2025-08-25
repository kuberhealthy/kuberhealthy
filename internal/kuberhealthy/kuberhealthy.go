package kuberhealthy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
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
	record "k8s.io/client-go/tools/record"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	restconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// defaultRunInterval is used when a check does not specify a runInterval or it fails to parse.
	defaultRunInterval = time.Minute * 10
	// scheduleLoopInterval controls how often Kuberhealthy scans for checks to run.
	scheduleLoopInterval = 30 * time.Second
)

// Kuberhealthy handles background processing for checks
type Kuberhealthy struct {
	Context     context.Context
	cancel      context.CancelFunc
	running     bool          // indicates that Start() has been called and this instance is running
	CheckClient client.Client // Kubernetes client for check CRUD

	Recorder record.EventRecorder // emits k8s events for khcheck lifecycle

	loopMu      sync.Mutex
	loopRunning bool
}

// New creates a new Kuberhealthy instance and event recorder
func New(ctx context.Context, checkClient client.Client) *Kuberhealthy {
	log.Infoln("New Kuberhealthy created")

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
			if err := khcrdsv2.AddToScheme(k8scheme.Scheme); err != nil {
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

// Start starts a new Kuberhealthy manager (this is the thing that kubebuilder makes)
// along with other various processes needed to manager Kuberhealthy checks.
func (kh *Kuberhealthy) Start(ctx context.Context) error {
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
	uList.SetGroupVersionKind(khcrdsv2.GroupVersion.WithKind("KuberhealthyCheckList"))
	if err := kh.CheckClient.List(kh.Context, uList); err != nil {
		log.Errorln("failed to list khchecks:", err)
		return
	}

	for _, khcheck := range uList.Items {
		runInterval := kh.runIntervalForCheck(&khcheck)

		var check khcrdsv2.KuberhealthyCheck
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(khcheck.Object, &check); err != nil {
			log.Errorf("failed to convert check %s/%s: %v", khcheck.GetNamespace(), khcheck.GetName(), err)
			continue
		}

		lastStart := time.Unix(check.Status.LastRunUnix, 0)
		if check.Status.CurrentUUID != "" {
			continue
		}
		if time.Since(lastStart) < runInterval {
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
func (kh *Kuberhealthy) StartCheck(khcheck *khcrdsv2.KuberhealthyCheck) error {
	log.Infoln("Starting Kuberhealthy check", khcheck.GetNamespace(), khcheck.GetName())
	if kh.Recorder != nil {
		// emit an event noting the check start
		kh.Recorder.Event(khcheck, corev1.EventTypeNormal, "CheckStarted", "check run started")
	}

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
	if err := kh.setLastRunTime(checkName, time.Now()); err != nil {
		return fmt.Errorf("unable to set check start time: %w", err)
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
	if kh.Recorder != nil {
		kh.Recorder.Eventf(khcheck, corev1.EventTypeNormal, "PodCreated", "created pod %s", podSpec.Name)
	}

	// write the name of the pod to the khcheck CRD's status
	if err := kh.setCheckPodName(checkName, podSpec.Name); err != nil {
		if kh.Recorder != nil {
			kh.Recorder.Event(khcheck, corev1.EventTypeWarning, "SetPodNameFailed", fmt.Sprintf("unable to set pod name: %v", err))
		}
		return fmt.Errorf("unable to set check pod name: %w", err)
	}
	return nil
}

// CheckPodSpec returns the corev1.PodSpec for this check's pods
func (kh *Kuberhealthy) CheckPodSpec(khcheck *khcrdsv2.KuberhealthyCheck) *corev1.Pod {

	// generate a random suffix and concatenate a unique pod name
	suffix := uuid.NewString()
	if len(suffix) > 5 {
		suffix = suffix[:5]
	}
	podName := fmt.Sprintf("%s-%s", khcheck.GetName(), suffix)

	// formulate a full pod spec
	podSpec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        podName,
			Namespace:   khcheck.GetNamespace(),
			Annotations: map[string]string{},
			Labels:      map[string]string{},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(khcheck, khcheck.GroupVersionKind()),
			},
		},
		Spec: khcheck.Spec.PodSpec.Spec,
	}

	// add required annotations
	podSpec.Annotations["createdBy"] = "kuberhealthy"
	podSpec.Annotations["kuberhealthyCheckName"] = khcheck.Name
	podSpec.Annotations["createdTime"] = time.Now().String()

	// add required labels
	podSpec.Labels["khcheck"] = khcheck.Name

	return podSpec
}

// StartCheck stops tracking and managing a khcheck. This occurs when a khcheck is removed.
func (kh *Kuberhealthy) StopCheck(khcheck *khcrdsv2.KuberhealthyCheck) error {
	log.Infoln("Stopping Kuberhealthy check", khcheck.GetNamespace(), khcheck.GetName())

	// clear CurrentUUID to indicate the check is no longer running
	checkName := createNamespacedName(khcheck.GetName(), khcheck.GetNamespace())
	kh.clearUUID(checkName)

	// calculate the run time and record it
	lastRunTime, err := kh.getLastRunTime(checkName)
	if err != nil {
		return err
	}

	// calculate the last run duration and store it
	runTime := time.Now().Sub(lastRunTime)
	err = kh.setRunDuration(checkName, runTime)
	if err != nil {
		return err
	}

	// fetch the current running pod's name
	podName, err := kh.getCurrentPodName(khcheck)
	if err != nil {
		return fmt.Errorf("failed to get check status for cleanup: %w", err)
	}

	// if the pod name is not set, we have nothing to do
	if podName == "" {
		return nil
	}

	// delete the checker pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: khcheck.GetNamespace(),
			Name:      podName,
		},
	}
	if err := kh.CheckClient.Delete(kh.Context, pod); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete checker pod %s: %w", podName, err)
		}
	}
	// clear stored pod name in status
	if err := kh.setCheckPodName(checkName, ""); err != nil {
		return fmt.Errorf("failed to clear checker pod name: %w", err)
	}

	return nil
}

// getCurrentPodName fetches the current pod's name for the provided khcheck from the control plane
func (kh *Kuberhealthy) getCurrentPodName(khcheck *khcrdsv2.KuberhealthyCheck) (string, error) {
	namespacedName := types.NamespacedName{
		Namespace: khcheck.GetNamespace(),
		Name:      khcheck.GetName(),
	}
	check, err := kh.getCheck(namespacedName)
	if err != nil {
		return "", fmt.Errorf("failed to get check status for cleanup: %w", err)
	}
	return check.Status.PodName, nil
}

// UpdateCheck handles the event of a check getting upcated in place
func (kh *Kuberhealthy) UpdateCheck(oldKHCheck *khcrdsv2.KuberhealthyCheck, newKHCheck *khcrdsv2.KuberhealthyCheck) error {
	log.Infoln("Updating Kuberhealthy check", oldKHCheck.GetNamespace(), oldKHCheck.GetName())
	return nil
}

// IsStarted returns if this instance is running or not
func (kh *Kuberhealthy) IsStarted() bool {
	return kh.running
}

// getCheck fetches a check based on its name and namespace
func (k *Kuberhealthy) getCheck(checkName types.NamespacedName) (*khcrdsv2.KuberhealthyCheck, error) {
	khCheck := &khcrdsv2.KuberhealthyCheck{}
	err := k.CheckClient.Get(k.Context, checkName, khCheck)
	return khCheck, err
}

// setCheckExecutionError sets an execution error for a khcheck in its crd status
func (k *Kuberhealthy) setCheckExecutionError(checkName types.NamespacedName, checkErrors []string) error {

	// get the check as it is right now
	khCheck, err := k.getCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	// set the errors
	khCheck.Status.Errors = checkErrors

	// update the khcheck resource
	err = k.CheckClient.Status().Update(k.Context, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update check errors with error: %w", err)
	}
	return nil
}

// setUUID sets the specified UUID on the check
func (k *Kuberhealthy) setUUID(checkName types.NamespacedName, uuid string) error {

	// get the check as it is right now
	khCheck, err := k.getCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	// set the errors
	khCheck.Status.CurrentUUID = uuid

	// update the khcheck resource
	err = k.CheckClient.Status().Update(k.Context, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update check uuid with error: %w", err)
	}
	return nil
}

// clearUUID clears the UUID assigned to the check, which indicates
// that it is not running.
func (k *Kuberhealthy) clearUUID(checkName types.NamespacedName) error {

	// get the check as it is right now
	khCheck, err := k.getCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	// set the errors
	khCheck.Status.CurrentUUID = ""

	// update the khcheck resource
	err = k.CheckClient.Status().Update(k.Context, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update check with fresh uuid with error: %w", err)
	}
	return nil
}

// setFreshUUID generates a UUID and sets it on the check.
func (k *Kuberhealthy) setFreshUUID(checkName types.NamespacedName) error {

	// get the check as it is right now
	khCheck, err := k.getCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	// set the errors
	khCheck.Status.CurrentUUID = uuid.NewString()

	// update the khcheck resource
	err = k.CheckClient.Status().Update(k.Context, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update check with fresh uuid with error: %w", err)
	}
	return nil
}

// setOK sets the OK property on the status of a khcheck
func (k *Kuberhealthy) setOK(checkName types.NamespacedName, ok bool) error {
	khCheck, err := k.getCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	khCheck.Status.OK = ok

	err = k.CheckClient.Status().Update(k.Context, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update OK status: %w", err)
	}

	return nil
}

// getLastRunTime gets the last run start time unix time
func (k *Kuberhealthy) getLastRunTime(checkName types.NamespacedName) (time.Time, error) {
	khCheck, err := k.getCheck(checkName)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get check: %w", err)
	}

	// secs is your Unix timestamp in seconds (int64)
	t := time.Unix(khCheck.Status.LastRunUnix, 0).UTC()
	return t, nil
}

// setLastRunTime sets the last run start time unix time
func (k *Kuberhealthy) setLastRunTime(checkName types.NamespacedName, lastRunTime time.Time) error {
	khCheck, err := k.getCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	khCheck.Status.LastRunUnix = lastRunTime.Unix()

	err = k.CheckClient.Status().Update(k.Context, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update LastRun time: %w", err)
	}

	return nil
}

// setRunDuration sets the time the last check took to run
func (k *Kuberhealthy) setRunDuration(checkName types.NamespacedName, runDuration time.Duration) error {
	khCheck, err := k.getCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	khCheck.Status.LastRunDuration = runDuration

	err = k.CheckClient.Status().Update(k.Context, khCheck)
	if err != nil {
		return fmt.Errorf("failed to update RunDuration: %w", err)
	}

	return nil
}

// setCheckPodName writes the name of the recently created checker pod to the check's status
func (k *Kuberhealthy) setCheckPodName(checkName types.NamespacedName, podName string) error {
	khCheck, err := k.getCheck(checkName)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}
	khCheck.Status.PodName = podName
	if err := k.CheckClient.Status().Update(k.Context, khCheck); err != nil {
		return fmt.Errorf("failed to update check pod name: %w", err)
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
	khCheck, err := k.getCheck(checkName)
	if err != nil {
		return "", fmt.Errorf("failed to get check: %w", err)
	}
	return khCheck.Status.CurrentUUID, nil
}
