package kuberhealthy

import (
	"context"
	"os"
	"sort"
	"strconv"
	"time"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// defaultRunTimeout is the amount of time a pod is allowed to run before the
// reaper considers it timed out.
const defaultRunTimeout = time.Minute * 5

// defaultFailedPodRetentionDays is the number of days to retain failed pods
// before they are reaped.
const defaultFailedPodRetentionDays = 4

// defaultMaxFailedPods is the maximum number of failed pods to retain for a
// check.
const defaultMaxFailedPods = 5

// runReaper periodically scans all khcheck pods and cleans up any that have
// exceeded their configured runtime or have lingered after completion.
func (kh *Kuberhealthy) runReaper(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := kh.reapOnce(); err != nil {
				log.Errorln("reaper:", err)
			}
		}
	}
}

// reapOnce performs a single scan of khchecks and applies cleanup logic. It is
// primarily exposed for unit testing.
func (kh *Kuberhealthy) reapOnce() error {
	var checkList khcrdsv2.KuberhealthyCheckList
	if err := kh.CheckClient.List(kh.Context, &checkList); err != nil {
		return err
	}

	// read configuration from environment variables
	retention := time.Duration(defaultFailedPodRetentionDays) * 24 * time.Hour
	if v := os.Getenv("KH_ERROR_POD_RETENTION_DAYS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			retention = time.Duration(i) * 24 * time.Hour
		}
	}
	maxFailed := defaultMaxFailedPods
	if v := os.Getenv("KH_MAX_ERROR_POD_COUNT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			maxFailed = i
		}
	}

	for i := range checkList.Items {
		check := &checkList.Items[i]
		checkNN := types.NamespacedName{Namespace: check.Namespace, Name: check.Name}

		// determine runtime parameters from the check spec if provided
		runTimeout := defaultRunTimeout
		runInterval := defaultRunInterval

		raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(check)
		if err == nil {
			if spec, ok := raw["spec"].(map[string]interface{}); ok {
				if v, ok := spec["runTimeout"].(string); ok {
					if d, err := time.ParseDuration(v); err == nil {
						runTimeout = d
					}
				}
				if v, ok := spec["runInterval"].(string); ok {
					if d, err := time.ParseDuration(v); err == nil {
						runInterval = d
					}
				}
			}
		}

		// list pods belonging to this check
		var podList corev1.PodList
		if err := kh.CheckClient.List(kh.Context, &podList,
			client.InNamespace(check.Namespace),
			client.MatchingLabels(map[string]string{"khcheck": check.Name}),
		); err != nil {
			log.Errorf("reaper: list pods for %s/%s: %v", check.Namespace, check.Name, err)
			continue
		}

		var failedPods []corev1.Pod

		// iterate over each pod and apply retention logic based on phase
		for pod := range podList.Items {
			podRef := &podList.Items[pod]
			age := time.Since(podRef.CreationTimestamp.Time)

			switch podRef.Status.Phase {
			case corev1.PodRunning, corev1.PodPending, corev1.PodUnknown:
				// terminate pods running longer than the allowed timeout
				if age > runTimeout {
					if err := kh.CheckClient.Delete(kh.Context, podRef); err != nil && !apierrors.IsNotFound(err) {
						log.Errorf("reaper: failed deleting timed out pod %s/%s: %v", podRef.Namespace, podRef.Name, err)
						continue
					}
					_ = kh.setCheckExecutionError(checkNN, []string{"check run timed out"})
					_ = kh.setOK(checkNN, false)
					_ = kh.clearUUID(checkNN)
					if check.Status.PodName == podRef.Name {
						_ = kh.setCheckPodName(checkNN, "")
					}
				}
			case corev1.PodSucceeded:
				// prune successful pods after three intervals
				if age > runInterval*3 {
					if err := kh.CheckClient.Delete(kh.Context, podRef); err != nil && !apierrors.IsNotFound(err) {
						log.Errorf("reaper: failed deleting completed pod %s/%s: %v", podRef.Namespace, podRef.Name, err)
						continue
					}
					if check.Status.PodName == podRef.Name {
						_ = kh.setCheckPodName(checkNN, "")
					}
				}
			case corev1.PodFailed:
				// accumulate failed pods for later pruning
				failedPods = append(failedPods, *podRef)
			default:
				log.Errorf("reaper: encountered pod %s/%s with unexpected phase %s", podRef.Namespace, podRef.Name, podRef.Status.Phase)
			}
		}

		// sort failed pods newest first
		sort.Slice(failedPods, func(i, j int) bool {
			return failedPods[i].CreationTimestamp.Time.After(failedPods[j].CreationTimestamp.Time)
		})

		// delete failed pods exceeding retention settings
		for pod := range failedPods {
			podRef := &failedPods[pod]
			age := time.Since(podRef.CreationTimestamp.Time)
			if pod >= maxFailed || age > retention {
				if err := kh.CheckClient.Delete(kh.Context, podRef); err != nil && !apierrors.IsNotFound(err) {
					log.Errorf("reaper: failed deleting failed pod %s/%s: %v", podRef.Namespace, podRef.Name, err)
					continue
				}
				if check.Status.PodName == podRef.Name {
					_ = kh.setCheckPodName(checkNN, "")
				}
			}
		}
	}
	return nil
}
