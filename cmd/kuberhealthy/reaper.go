package main

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	khcrds "github.com/kuberhealthy/crds/api/v1"
	"github.com/kuberhealthy/kuberhealthy/v3/pkg/kubeclient"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

// Default values for reaper configurations
const minKHJobAge = time.Minute * 5
const minCheckPodAge = time.Second * 30
const checkReaperRunIntervalDefault = time.Second * 30

type Reaper struct {
	// ReapCheckerPods is a variable mapping all reaper pods
	ReapCheckerPods map[string]v1.Pod
}

func NewReaper() *Reaper {
	return &Reaper{
		ReapCheckerPods: make(map[string]v1.Pod),
	}
}

// parseDurationOrUseDefault parses a string duration into a time.Duration. If string is empty, return the defaultDuration.
// If the parsed time.Duration is 0, return defaultDuration.
func (r *Reaper) parseDurationOrUseDefault(d string, defaultDuration time.Duration) (time.Duration, error) {

	if len(d) == 0 {
		return defaultDuration, nil
	}

	duration, err := time.ParseDuration(d)
	if err != nil {
		return defaultDuration, err
	}

	if duration == 0 {
		log.Errorln("checkReaper: duration value 0 is not valid")
		log.Infoln("checkReaper: Using default duration:", defaultDuration)
		return defaultDuration, nil
	}

	return duration, nil

}

// Runr runs the reaper until the supplied context expires and reaps khjobs and khchecks.  To target all
// namespace, set the namespace string to ""
func (r *Reaper) Run(ctx context.Context, namespace string) {

	reaperRunInterval, err := r.parseDurationOrUseDefault(checkReaperRunInterval, checkReaperRunIntervalDefault)
	if err != nil {
		log.Errorln("checkReaper: Error occurred attempting to parse checkReaperRunInterval:", err)
		log.Infoln("checkReaper: Using default checkReaperRunInterval:", checkReaperRunIntervalDefault)
	}

	// Parse configs when reaper starts up.
	log.Infoln("checkReaper: starting up...")
	log.Infoln("checkReaper: run interval:", reaperRunInterval)
	log.Infoln("checkReaper: max khjob age:", GlobalConfig.MaxKHJobAge)
	log.Infoln("checkReaper: max khcheck pod age:", GlobalConfig.MaxCheckPodAge)
	log.Infoln("checkReaper: max completed check pod count:", GlobalConfig.MaxCompletedPodCount)
	log.Infoln("checkReaper: max error check pod count:", GlobalConfig.MaxErrorPodCount)

	// set MaxCheckPodAge to minCheckPodAge before getting reaped if no maxCheckPodAge is set
	// Want to make sure the completed pod is around for at least 30s before getting reaped
	if GlobalConfig.MaxCheckPodAge < minCheckPodAge {
		GlobalConfig.MaxCheckPodAge = minCheckPodAge
	}

	// set MaxKHJobAge to minKHJobAge before getting reaped if no maxCheckPodAge is set
	// Want to make sure the completed job is around for at least 5m before getting reaped
	if GlobalConfig.MaxKHJobAge < minKHJobAge {
		GlobalConfig.MaxKHJobAge = minKHJobAge
	}

	// start a new ticker
	t := time.NewTicker(reaperRunInterval)
	defer t.Stop()

	// iterate until our context expires and run reaper operations
	keepGoing := true
	for keepGoing {
		<-t.C

		// create a context for this run that times out
		runCtx, runCtxCancel := context.WithTimeout(ctx, time.Minute*3)
		defer runCtxCancel()

		// run our check and job reapers
		r.runCheckReap(runCtx, namespace)
		r.runJobReap(runCtx, namespace)

		// check if the parent context has expired
		select {
		case <-ctx.Done():
			log.Debugln("checkReaper: context has expired...")
			keepGoing = false
		default:
		}
	}

	log.Infoln("checkReaper: check reaper shutting down...")
}

// runCheckReap runs a process which locates checkpods that need reaped and reaps them
func (r *Reaper) runCheckReap(ctx context.Context, namespace string) {

	// list checker pods in all namespaces
	podList, err := r.listCompletedCheckerPods(ctx, namespace)
	if err != nil {
		log.Errorln("checkReaper: Failed to list and delete old checker pods", err)
	}

	if len(podList) == 0 {
		log.Infoln("checkReaper: No pods found that need reaped.")
		return
	}

	err = r.deleteFilteredCheckerPods(ctx, podList)
	if err != nil {
		log.Errorln("checkReaper: Error found while deleting old pods:", err)
	}

	log.Infoln("checkReaper: Finished reaping checker pods.")

}

// runJobReap runs a process to reap jobs that need deleted (those that were created by a khjob)
func (r *Reaper) runJobReap(ctx context.Context, namespace string) {

	log.Infoln("checkReaper: Beginning to search for khjobs.")
	// fetch and delete khjobs that meet criteria
	err := r.khJobDelete(ctx, namespace)
	if err != nil {
		log.Errorln("checkReaper: Failed to reap khjobs with error: ", err)
	}
	log.Infoln("checkReaper: Finished reaping khjobs.")
}

// listCompletedCheckerPods returns a list of completed (Failed of Succeeded) pods with the khcheck name label
func (r *Reaper) listCompletedCheckerPods(ctx context.Context, namespace string) (map[string]v1.Pod, error) {
	log.Infoln("checkReaper: Listing checker pods")

	r.ReapCheckerPods = make(map[string]v1.Pod) // pods to reap

	// fetch all pods with the label kuberhealthy-check-name in the supplied namespace
	podList := &v1.PodList{}
	listOpts := []client.ListOption{client.InNamespace(namespace), client.HasLabels{"kuberhealthy-check-name"}}
	err := KubernetesClient.Client.List(ctx, podList, listOpts...)
	if err != nil {
		log.Errorln("checkReaper: Failed to list checker pods")
		return r.ReapCheckerPods, err
	}

	log.Infoln("checkReaper: Found:", len(podList.Items), "checker pods")

	for _, p := range podList.Items {
		if p.Status.Phase == v1.PodSucceeded || p.Status.Phase == v1.PodFailed {
			r.ReapCheckerPods[p.Namespace+"/"+p.Name] = p
		}
	}

	return r.ReapCheckerPods, err
}

// deleteFilteredCheckerPods goes through map of all checker pods and deletes older checker pods
func (r *Reaper) deleteFilteredCheckerPods(ctx context.Context, reapCheckerPods map[string]v1.Pod) error {

	var err error

	for n, p := range reapCheckerPods {

		podTerminatedTime, err := r.getPodCompletedTime(p)
		if err != nil {
			log.Warnln(err)
			continue
		}
		// Delete pods older than maxCheckPodAge and is in status Succeeded
		if p.Status.Phase == v1.PodSucceeded && time.Since(podTerminatedTime) > GlobalConfig.MaxCheckPodAge {
			log.Infoln("checkReaper: Found completed pod older than:", GlobalConfig.MaxCheckPodAge, "in status `Succeeded`. Deleting pod:", p)

			err = KubernetesClient.Client.Delete(ctx, &p)
			// err = KubernetesClient.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Errorln("checkReaper: Failed to delete pod:", p, err)
				continue
			}
			delete(reapCheckerPods, n)
		}

		// Delete failed pods (status Failed) older than maxCheckPodAge
		if p.Status.Phase == v1.PodFailed && time.Since(podTerminatedTime) > GlobalConfig.MaxCheckPodAge {
			log.Infoln("checkReaper: Found completed pod older than:", GlobalConfig.MaxCheckPodAge, "in status `Failed`. Deleting pod:", p.Name)

			err = KubernetesClient.Client.Delete(ctx, &p)
			// err = KubernetesClient.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Errorln("checkReaper: Failed to delete pod:", p, err)
				continue
			}
			delete(reapCheckerPods, n)
		}

		// Delete if there are more than MaxCompletedPodCount checker pods with the same name in status Succeeded that were created more recently
		// Delete if the checker pod is Failed and there are more than MaxErrorPodCount checker pods of the same type which were created more recently
		checkName := p.Annotations["comcast.github.io/check-name"]
		allCheckPods := r.getAllCompletedPodsWithCheckName(reapCheckerPods, checkName)
		if len(allCheckPods) > GlobalConfig.MaxCompletedPodCount {

			failOldCount := 0
			failCount := 0
			successOldCount := 0
			successCount := 0
			for _, p := range allCheckPods {
				if p.CreationTimestamp.Time.Before(p.CreationTimestamp.Time) && p.Status.Phase != v1.PodSucceeded {
					failOldCount++
				}
				if p.Status.Phase != v1.PodSucceeded {
					failCount++
				}
				if p.CreationTimestamp.Time.Before(p.CreationTimestamp.Time) && p.Status.Phase == v1.PodSucceeded {
					successOldCount++
				}
				if p.Status.Phase == v1.PodSucceeded {
					successCount++
				}
			}

			// Delete if there are more than MaxCompletedPodCount checker pods with the same name in status Succeeded that were created more recently
			if p.Status.Phase == v1.PodSucceeded && successOldCount >= GlobalConfig.MaxCompletedPodCount && successCount >= GlobalConfig.MaxCompletedPodCount {
				log.Infoln("checkReaper: Found more than", GlobalConfig.MaxCompletedPodCount, "checker pods with the same name in status `Succeeded` that were created more recently. Deleting pod:", n)

				err = r.deletePod(ctx, p)
				if err != nil {
					log.Errorln("checkReaper: Failed to delete pod:", n, err)
					continue
				}
				delete(reapCheckerPods, n)
			}

			// Delete if there are more than MaxErrorPodCount checker pods with the same name in status Failed that were created more recently
			if p.Status.Phase == v1.PodFailed && failOldCount >= GlobalConfig.MaxErrorPodCount && failCount >= GlobalConfig.MaxErrorPodCount {
				log.Infoln("checkReaper: Found more than", GlobalConfig.MaxErrorPodCount, "checker pods with the same name in status Failed` that were created more recently. Deleting pod:", n)

				err = r.deletePod(ctx, p)
				if err != nil {
					log.Errorln("checkReaper: Failed to delete pod:", n, err)
					continue
				}
				delete(reapCheckerPods, n)
			}
		}
	}
	return err
}

// getAllCompletedPodsWithCheckName finds all completed checker pods that are of the same source check as the provided pod
func (r *Reaper) getAllCompletedPodsWithCheckName(reapCheckerPods map[string]v1.Pod, checkName string) []v1.Pod {

	var allCheckPods []v1.Pod

	// checkName := pod.Annotations["comcast.github.io/check-name"]

	for _, v := range reapCheckerPods {
		if v.Labels["kuberhealthy-check-name"] == checkName {
			podTerminatedTime, err := r.getPodCompletedTime(v)
			if err != nil {
				log.Warnln(err)
				continue
			}
			if time.Since(podTerminatedTime) > minCheckPodAge {
				allCheckPods = append(allCheckPods, v)
			}
		}
	}

	return allCheckPods
}

// deletePod deletes a given pod
func (r *Reaper) deletePod(ctx context.Context, pod v1.Pod) error {
	log.Infoln("checkReaper: Deleting Pod: ", pod.Name, " in namespace: ", pod.Namespace)
	return KubernetesClient.Client.Delete(ctx, &pod, kubeclient.ForegroundDeleteOption())
}

// jobConditions returns true if conditions are met to be deleted for khjob
func (r *Reaper) jobConditions(job khcrds.KuberhealthyJob, duration time.Duration, phase khcrds.JobPhase) bool {
	if time.Since(job.CreationTimestamp.Time) > duration && job.Spec.Phase == phase {
		log.Infoln("checkReaper: Found khjob older than", duration, "minutes in status", phase)
		return true
	}
	return false
}

// khJobDelete fetches a list of khjobs in a namespace and will delete them if they meet specific criteria
func (r *Reaper) khJobDelete(ctx context.Context, namespace string) error {

	opts := metav1.ListOptions{}
	delOpts := metav1.DeleteOptions{}

	// list khjobs in Namespace
	list, err := KubernetesClient.ListKuberhealthyJobs(namespace, &opts)
	if err != nil {
		log.Errorln("checkReaper: Error: failed to retrieve khjob list with error", err)
		return err
	}

	log.Infoln("checkReaper: Found", len(list.Items), "khjobs")

	// Range over list and delete khjobs
	for _, j := range list.Items {
		if r.jobConditions(j, GlobalConfig.MaxKHJobAge, "Completed") {
			log.Infoln("checkReaper: Deleting khjob", j.Name)
			err = KubernetesClient.DeleteKuberhealthyJob(j.Name, j.Namespace, &delOpts)
			if err != nil {
				log.Errorln("checkReaper: Failure to delete khjob", j.Name, "with error:", err)
				return err

			}
		}
	}
	return nil
}

// getPodCompletedTime returns a boolean to ensure container terminated state exists and returns containers' latest finished time
func (r *Reaper) getPodCompletedTime(pod v1.Pod) (time.Time, error) {

	var podCompletedTime time.Time
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil {
			finishedTime := cs.State.Terminated.FinishedAt
			if finishedTime.After(podCompletedTime) {
				podCompletedTime = finishedTime.Time
			}
		} else {
			return podCompletedTime, fmt.Errorf("could not fetch pod: %s completed time", pod.Name)
		}
	}

	return podCompletedTime, nil
}
