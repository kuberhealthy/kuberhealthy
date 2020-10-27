package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	khjobcrd "github.com/Comcast/kuberhealthy/v2/pkg/apis/khjob/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
)

// kubeConfigFile is a variable containing file path of Kubernetes config files
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// ReapCheckerPods is a variable mapping all reaper pods
var ReapCheckerPods map[string]v1.Pod

// MaxPodsThresholdEnv is a variable limiting how many reaper pods can exist in a cluster
var MaxPodsThresholdEnv = os.Getenv("MAX_PODS_THRESHOLD")

// JobDeleteTimeDurationEnv is a variable limiting how many minutes a khjob can be alive before it can be delted
var JobDeleteTimeDurationEnv = os.Getenv("JOB_DELETE_TIME_DURATION")

// instantiate kuberhealhty job client CRD
var khJobClient *khjobcrd.KHJobV1Client

// Namespace is a variable to allow code to target all namespaces or a single namespace
var Namespace string

func init() {
	Namespace = os.Getenv("SINGLE_NAMESPACE")
	if len(Namespace) == 0 {
		log.Infoln("Single namespace not specified, running check reaper across all namespaces")
		Namespace = ""
	} else {
		log.Infoln("Single namespace specified. Running check-reaper in namespace:", Namespace)
	}
}

func main() {
	ctx := context.Background()

	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	jobClient, err := khjobcrd.Client(kubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create khJob client", err)
	}

	podList, err := listCheckerPods(ctx, client, Namespace)
	if err != nil {
		log.Fatalln("Failed to list and delete old checker pods", err)
	}

	if len(podList) == 0 {
		log.Infoln("No pods found.")
		return
	}

	err = deleteFilteredCheckerPods(ctx, client, podList)
	if err != nil {
		log.Fatalln("Error found while deleting old pods:", err)
	}

	log.Infoln("Finished reaping checker pods.")
	log.Infoln("Beginning to search for khjobs.")

	// fetch and delete khjobs that meet criteria
	err = khJobDelete(jobClient)
	if err != nil {
		log.Errorln("Failed to reap khjobs with error: ", err)
	}
	log.Infoln("Finished reaping khjobs.")
}

// listCheckerPods returns a list of pods with the khcheck name label
func listCheckerPods(ctx context.Context, client *kubernetes.Clientset, namespace string) (map[string]v1.Pod, error) {
	log.Infoln("Listing checker pods")

	ReapCheckerPods = make(map[string]v1.Pod)

	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "kuberhealthy-check-name"})
	if err != nil {
		log.Errorln("Failed to list checker pods")
		return ReapCheckerPods, err
	}

	log.Infoln("Found:", len(pods.Items), "checker pods")

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

	MaxPodsThreshold, err := strconv.Atoi(MaxPodsThresholdEnv)
	if err != nil {
		log.Errorln("Error converting MaxPodsThreshold to int")
	}

	for k, v := range reapCheckerPods {

		// Delete pods older than 5 hours and is in status Succeeded
		if time.Now().Sub(v.CreationTimestamp.Time).Hours() > 5 && v.Status.Phase == v1.PodSucceeded {
			log.Infoln("Found pod older than 5 hours in status `Succeeded`. Deleting pod:", k)

			err = deletePod(ctx, client, v)
			if err != nil {
				log.Errorln("Failed to delete pod:", k, err)
				continue
			}
			delete(reapCheckerPods, k)
		}

		// Delete failed pods (status Failed) older than 5 days (120 hours)
		if time.Now().Sub(v.CreationTimestamp.Time).Hours() > 120 && v.Status.Phase == v1.PodFailed {
			log.Infoln("Found pod older than 5 days in status `Failed`. Deleting pod:", k)

			err = deletePod(ctx, client, v)
			if err != nil {
				log.Errorln("Failed to delete pod:", k, err)
				continue
			}
			delete(reapCheckerPods, k)
		}

		// Delete if there are more than 5 checker pods with the same name in status Succeeded that were created more recently
		// Delete if the checker pod is Failed and there are more than 5 Failed checker pods of the same type which were created more recently
		allCheckPods := getAllPodsWithCheckName(reapCheckerPods, v)
		if len(allCheckPods) > MaxPodsThreshold {

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

			// Delete if there are more than 5 checker pods with the same name in status Succeeded that were created more recently
			if v.Status.Phase == v1.PodSucceeded && successOldCount > MaxPodsThreshold && successCount > MaxPodsThreshold {
				log.Infoln("Found more than 5 checker pods with the same name in status `Succeeded` that were created more recently. Deleting pod:", k)

				err = deletePod(ctx, client, v)
				if err != nil {
					log.Errorln("Failed to delete pod:", k, err)
					continue
				}
				delete(reapCheckerPods, k)
			}

			// Delete if the checker pod is Failed and there are more than 5 Failed checker pods of the same type which were created more recently
			if v.Status.Phase == v1.PodFailed && failOldCount > MaxPodsThreshold && failCount > MaxPodsThreshold {
				log.Infoln("Found more than 5 `Failed` checker pods of the same type which were created more recently. Deleting pod:", k)

				err = deletePod(ctx, client, v)
				if err != nil {
					log.Errorln("Failed to delete pod:", k, err)
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

	log.Infoln("Deleting Pod: ", pod.Name, " in namespace: ", pod.Namespace)
	propagationForeground := metav1.DeletePropagationForeground
	options := metav1.DeleteOptions{PropagationPolicy: &propagationForeground}
	return client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, options)
}

// jobConditions returns true if conditions are met to be deleted for khjob
func jobConditions(job khjobcrd.KuberhealthyJob, duration time.Duration, phase khjobcrd.JobPhase) bool {
	if time.Now().Sub(job.CreationTimestamp.Time) > duration && job.Spec.Phase == phase {
		log.Infoln("Found khjob older than", duration, "minutes in status", phase)
		return true
	}
	return false
}

// KHJobDelete fetches a list of khjobs in a namespace and will delete them if they meet given criteria
func khJobDelete(client *khjobcrd.KHJobV1Client) error {

	opts := metav1.ListOptions{}
	del := metav1.DeleteOptions{}

	// convert JobDeleteMinutes into time.Duration
	jobDeleteTimeDuration, err := time.ParseDuration(JobDeleteTimeDurationEnv)
	if err != nil {
		log.Errorln("Error converting JobDeleteTimeDurationEnv to Float")
		return err
	}

	// list khjobs in Namespace
	list, err := client.KuberhealthyJobs(Namespace).List(opts)
	if err != nil {
		log.Errorln("Error: failed to retrieve khjob list with error", err)
		return err
	}

	log.Infoln("Found", len(list.Items), "khjobs")

	// Range over list and delete khjobs
	for _, j := range list.Items {
		if jobConditions(j, jobDeleteTimeDuration, "Completed") {
			log.Infoln("Deleting khjob", j.Name)
			err := client.KuberhealthyJobs(j.Namespace).Delete(j.Name, &del)
			if err != nil {
				log.Errorln("Failure to delete khjob", j.Name, "with error:", err)
				return err

			}
		}
	}
	return nil
}
