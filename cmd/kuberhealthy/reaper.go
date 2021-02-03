// Copyright 2018 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	khjobcrd "github.com/Comcast/kuberhealthy/v2/pkg/apis/khjob/v1"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

// ReapCheckerPods is a variable mapping all reaper pods
var ReapCheckerPods map[string]v1.Pod

// Default values for reaper configurations
const maxCheckerPodsDefault = 4
const jobCleanupDurationDefault = time.Minute * 15

func init() {

	// Set default MaxCheckPods and JobCleanupDuration if its not set in the Kuberhealthy ConfigMap
	if cfg.MaxCheckPods == 0 {
		cfg.MaxCheckPods = maxCheckerPodsDefault
	}

	if cfg.JobCleanupDuration == 0 {
		cfg.JobCleanupDuration = jobCleanupDurationDefault
	}
}

// reaper runs until the supplied context expires and reaps khjobs and khchecks
func reaper(ctx context.Context) {

	log.Infoln("checkReaper: starting up...")
	log.Infoln("checkReaper: job cleanup duration:", cfg.JobCleanupDuration)
	log.Infoln("checkReaper: max checker pods:", cfg.MaxCheckPods)

	// start a new ticker
	t := time.NewTicker(time.Minute * 3)
	defer t.Stop()

	// iterate until our context expires and run reaper operations
	keepGoing := true
	for keepGoing {
		<-t.C

		// create a context for this run that times out
		runCtx, runCtxCancel := context.WithTimeout(ctx, time.Minute*3)
		defer runCtxCancel()

		// run our check and job reapers
		runCheckReap(runCtx)
		runJobReap(runCtx)

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
func runCheckReap(ctx context.Context) {

	// list checker pods in all namespaces
	podList, err := listCheckerPods(ctx, kubernetesClient, listenNamespace)
	if err != nil {
		log.Errorln("checkReaper: Failed to list and delete old checker pods", err)
	}

	if len(podList) == 0 {
		log.Infoln("checkReaper: No pods found that need reaped.")
		return
	}

	err = deleteFilteredCheckerPods(ctx, kubernetesClient, podList)
	if err != nil {
		log.Errorln("checkReaper: Error found while deleting old pods:", err)
	}

	log.Infoln("checkReaper: Finished reaping checker pods.")

}

// runJobReap runs a process to reap jobs that need deleted (those that were created by a khjob)
func runJobReap(ctx context.Context) {
	jobClient, err := khjobcrd.Client(cfg.kubeConfigFile)
	if err != nil {
		log.Errorln("checkReaper: Unable to create khJob client", err)
	}

	log.Infoln("checkReaper: Beginning to search for khjobs.")
	// fetch and delete khjobs that meet criteria
	err = khJobDelete(jobClient)
	if err != nil {
		log.Errorln("checkReaper: Failed to reap khjobs with error: ", err)
	}
	log.Infoln("checkReaper: Finished reaping khjobs.")
}

// listCheckerPods returns a list of pods with the khcheck name label
func listCheckerPods(ctx context.Context, client *kubernetes.Clientset, namespace string) (map[string]v1.Pod, error) {
	log.Infoln("checkReaper: Listing checker pods")

	ReapCheckerPods = make(map[string]v1.Pod)

	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "kuberhealthy-check-name"})
	if err != nil {
		log.Errorln("checkReaper: Failed to list checker pods")
		return ReapCheckerPods, err
	}

	log.Infoln("checkReaper: Found:", len(pods.Items), "checker pods")

	for _, p := range pods.Items {
		if p.Status.Phase == v1.PodSucceeded || p.Status.Phase == v1.PodFailed {
			//log.Infoln("Checker pod: ", p.Name, "found in namespace: ", p.Namespace)
			ReapCheckerPods[p.Name] = p
		}
	}

	return ReapCheckerPods, err
}

// deleteFilteredCheckerPods goes through map of all checker pods and deletes older checker pods
func deleteFilteredCheckerPods(ctx context.Context, client *kubernetes.Clientset, reapCheckerPods map[string]v1.Pod) error {

	var err error

	for k, v := range reapCheckerPods {

		// Delete pods older than 5 hours and is in status Succeeded
		if time.Now().Sub(v.CreationTimestamp.Time).Hours() > 5 && v.Status.Phase == v1.PodSucceeded {
			log.Infoln("checkReaper: Found pod older than 5 hours in status `Succeeded`. Deleting pod:", k)

			err = deletePod(ctx, client, v)
			if err != nil {
				log.Errorln("checkReaper: Failed to delete pod:", k, err)
				continue
			}
			delete(reapCheckerPods, k)
		}

		// Delete failed pods (status Failed) older than 5 days (120 hours)
		if time.Now().Sub(v.CreationTimestamp.Time).Hours() > 120 && v.Status.Phase == v1.PodFailed {
			log.Infoln("checkReaper: Found pod older than 5 days in status `Failed`. Deleting pod:", k)

			err = deletePod(ctx, client, v)
			if err != nil {
				log.Errorln("checkReaper: Failed to delete pod:", k, err)
				continue
			}
			delete(reapCheckerPods, k)
		}

		// Delete if there are more than MaxPodsThreshold checker pods with the same name in status Succeeded that were created more recently
		// Delete if the checker pod is Failed and there are more than MaxPodsThreshold Failed checker pods of the same type which were created more recently
		allCheckPods := getAllPodsWithCheckName(reapCheckerPods, v)
		if len(allCheckPods) > cfg.MaxCheckPods {

			failOldCount := 0
			failCount := 0
			successOldCount := 0
			successCount := 0
			for _, p := range allCheckPods {
				if v.CreationTimestamp.Time.Before(p.CreationTimestamp.Time) && p.Status.Phase != v1.PodSucceeded && v.Namespace == p.Namespace {
					failOldCount++
				}
				if p.Status.Phase != v1.PodSucceeded && v.Namespace == p.Namespace {
					failCount++
				}
				if v.CreationTimestamp.Time.Before(p.CreationTimestamp.Time) && p.Status.Phase == v1.PodSucceeded && v.Namespace == p.Namespace {
					successOldCount++
				}
				if p.Status.Phase == v1.PodSucceeded && v.Namespace == p.Namespace {
					successCount++
				}
			}

			// Delete if there are more than MaxPodsThreshold checker pods with the same name in status Succeeded that were created more recently
			if v.Status.Phase == v1.PodSucceeded && successOldCount > cfg.MaxCheckPods && successCount > cfg.MaxCheckPods {
				log.Infoln("checkReaper: Found more than", cfg.MaxCheckPods, "checker pods with the same name in status `Succeeded` that were created more recently. Deleting pod:", k)

				err = deletePod(ctx, client, v)
				if err != nil {
					log.Errorln("checkReaper: Failed to delete pod:", k, err)
					continue
				}
				delete(reapCheckerPods, k)
			}

			// Delete if there are more than MaxPodsThreshold checker pods with the same name in status Failed that were created more recently
			if v.Status.Phase == v1.PodFailed && failOldCount > cfg.MaxCheckPods && failCount > cfg.MaxCheckPods {
				log.Infoln("checkReaper: Found more than", cfg.MaxCheckPods, "checker pods with the same name in status Failed` that were created more recently. Deleting pod:", k)

				err = deletePod(ctx, client, v)
				if err != nil {
					log.Errorln("checkReaper: Failed to delete pod:", k, err)
					continue
				}
				delete(reapCheckerPods, k)
			}
		}
	}
	return err
}

// getAllPodsWithCheckName finds all checker pods for a given khcheck
func getAllPodsWithCheckName(reapCheckerPods map[string]v1.Pod, pod v1.Pod) []v1.Pod {

	var allCheckPods []v1.Pod

	checkName := pod.Annotations["comcast.github.io/check-name"]

	for _, v := range reapCheckerPods {
		if v.Labels["kuberhealthy-check-name"] == checkName {
			allCheckPods = append(allCheckPods, v)
		}
	}

	return allCheckPods
}

// deletePod deletes a given pod
func deletePod(ctx context.Context, client *kubernetes.Clientset, pod v1.Pod) error {

	log.Infoln("checkReaper: Deleting Pod: ", pod.Name, " in namespace: ", pod.Namespace)
	propagationForeground := metav1.DeletePropagationForeground
	options := metav1.DeleteOptions{PropagationPolicy: &propagationForeground}
	return client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, options)
}

// jobConditions returns true if conditions are met to be deleted for khjob
func jobConditions(job khjobcrd.KuberhealthyJob, duration time.Duration, phase khjobcrd.JobPhase) bool {
	if time.Now().Sub(job.CreationTimestamp.Time) > duration && job.Spec.Phase == phase {
		log.Infoln("checkReaper: Found khjob older than", duration, "minutes in status", phase)
		return true
	}
	return false
}

// KHJobDelete fetches a list of khjobs in a namespace and will delete them if they meet given criteria
func khJobDelete(client *khjobcrd.KHJobV1Client) error {

	opts := metav1.ListOptions{}
	del := metav1.DeleteOptions{}

	// list khjobs in Namespace
	list, err := client.KuberhealthyJobs(listenNamespace).List(opts)
	if err != nil {
		log.Errorln("checkReaper: Error: failed to retrieve khjob list with error", err)
		return err
	}

	log.Infoln("checkReaper: Found", len(list.Items), "khjobs")

	// Range over list and delete khjobs
	for _, j := range list.Items {
		if jobConditions(j, cfg.JobCleanupDuration, "Completed") {
			log.Infoln("checkReaper: Deleting khjob", j.Name)
			err := client.KuberhealthyJobs(j.Namespace).Delete(j.Name, &del)
			if err != nil {
				log.Errorln("checkReaper: Failure to delete khjob", j.Name, "with error:", err)
				return err

			}
		}
	}
	return nil
}
