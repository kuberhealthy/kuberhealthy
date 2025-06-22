package kuberhealthy

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// Kuberhealthy handles background processing for checks
type Kuberhealthy struct {
	Context     context.Context
	Running     bool          // indicates that Start() has been called and this instance is running
	CheckClient client.Client // Kubernetes client for check CRUD
}

// New creates a new Kuberhealthy instance
func New(ctx context.Context, checkClient client.Client) *Kuberhealthy {
	log.Println("New Kuberhealthy created")
	return &Kuberhealthy{
		Context:     ctx,
		CheckClient: checkClient,
	}
}

// Start starts a new Kuberhealthy manager (this is the thing that kubebuilder makes)
// along with other various processes needed to manager Kuberhealthy checks.
func (kh *Kuberhealthy) Start(ctx context.Context) error {
	if kh.IsStarted() {
		return fmt.Errorf("error: kuberhealthy main controller was started but it was already running")
	}
	log.Println("Kuberhealthy start")
	return nil
}

// StartCheck begins tracking and managing a khcheck
func (kh *Kuberhealthy) StartCheck(khcheck *khcrdsv2.KuberhealthyCheck) error {
	log.Println("Starting Kuberhealthy check", khcheck.GetNamespace(), khcheck.GetName())

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
		return fmt.Errorf("failed to create check pod: %w", err)
	}

	return nil
}

// CheckPodSpec returns the corev1.PodSpec for this check's pods
func (kh *Kuberhealthy) CheckPodSpec(khcheck *khcrdsv2.KuberhealthyCheck) *corev1.Pod {

	// formulate a full pod spec
	podSpec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    khcheck.GetNamespace(),
			GenerateName: khcheck.GetName(),
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

// StartCheck stops tracking and managing a khcheck
func (kh *Kuberhealthy) StopCheck(khcheck *khcrdsv2.KuberhealthyCheck) error {
	log.Println("Stopping Kuberhealthy check", khcheck.GetNamespace(), khcheck.GetName())

	checkName := types.NamespacedName{
		Namespace: khcheck.GetNamespace(),
		Name:      khcheck.GetName(),
	}

	// clear CurrentUUID to indicate the check is no longer running
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

	// TODO: additional cleanup logic here

	return nil
}

// UpdateCheck handles the event of a check getting upcated in place
func (kh *Kuberhealthy) UpdateCheck(oldKHCheck *khcrdsv2.KuberhealthyCheck, newKHCheck *khcrdsv2.KuberhealthyCheck) error {
	log.Println("Updating Kuberhealthy check", oldKHCheck.GetNamespace(), oldKHCheck.GetName())
	return nil
}

// IsStarted returns if this instance is running or not
func (kh *Kuberhealthy) IsStarted() bool {
	return kh.Running
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
