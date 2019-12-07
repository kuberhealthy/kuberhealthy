package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
)

var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var ReapEvictedPods map[string]*v1.Pod
var MaxPodsThreshold = 5

func main() {

	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	podList, err := listEvictedPods(client)
	if err != nil {
		log.Fatalln("Failed to list evicted pods", err)
	}

	if len(podList) == 0 {
		fmt.Println("No evicted pods found.")
		return
	}

	err = deleteFilteredEvictedPods(client, podList)
	if err != nil {
		log.Fatalln("Error found while deleting old evicted pods:", err)
	}
}

// listEvictedPods returns a list of evicted pods
func listEvictedPods(client *kubernetes.Clientset) (map[string]*v1.Pod, error) {
	fmt.Println("Listing checker pods from all namespaces")

	ReapEvictedPods = make(map[string]*v1.Pod)

	pods, err := client.CoreV1().Pods("").List(metav1.ListOptions{LabelSelector: "kuberhealthy-check-name"})
	if err != nil {
		log.Errorln("Failed to list checker pods from all namespaces")
		return ReapEvictedPods, err
	}
	for _, p := range pods.Items {
		if p.Status.Reason == "Evicted" {
			fmt.Println("\tEvicted checker pod: ", p.Name, "found in namespace: ", p.Namespace)
			ReapEvictedPods[p.Name] = &p
		}
	}

	return ReapEvictedPods, err
}

// filterEvictedPods maintains a map of checker pods to be deleted. Process of elimination.
func deleteFilteredEvictedPods(client *kubernetes.Clientset, reapEvictedPods map[string]*v1.Pod) error {

	var err error

	for k, v := range reapEvictedPods {

		log.Infoln("Filtering evicted checker pods")

		// Delete pods older than 5 hours and is in status Succeeded
		if time.Now().Sub(v.CreationTimestamp.Time).Hours() > 5 && v.Status.Phase == "Succeeded" {
			log.Infoln("Found pod older than 5 hours in status Succeeded`. Deleting pod:", k)

			err = deleteEvictedPod(client, v)
			if err != nil {
				log.Errorln("Failed to delete evicted pod:", k, err)
				return err
			}
			delete(reapEvictedPods, k)
		}

		// Delete failed pods (status Failed) older than 5 days (120 hours)
		if time.Now().Sub(v.CreationTimestamp.Time).Hours() > 120 && v.Status.Phase == "Failed" {
			log.Infoln("Found pod older than 5 days in status `Failed`. Deleting pod:", k)

			err = deleteEvictedPod(client, v)
			if err != nil {
				log.Errorln("Failed to delete evicted pod:", k, err)
				return err
			}
			delete(reapEvictedPods, k)
		}

		// Delete if there are more than 5 checker pods with the same name in status Succeeded that were created more recently
		// Delete if the checker pod is Failed and there are more than 5 Failed checker pods of the same type which were created more recently
		allCheckPods := getAllPodsWithCheckName(reapEvictedPods, v)
		if len(allCheckPods) > MaxPodsThreshold {

			oldCount := 0
			failedCount := 0
			for _, p := range allCheckPods {
				if v.CreationTimestamp.Time.Before(p.CreationTimestamp.Time) {
					oldCount++
				}
				if p.Status.Phase != "Succeeded" {
					failedCount++
				}
			}

			// Delete if there are more than 5 checker pods with the same name in status Succeeded that were created more recently
			if oldCount > MaxPodsThreshold && failedCount == 0 {
				err = deleteEvictedPod(client, v)
				if err != nil {
					log.Errorln("Failed to delete evicted pod:", k, err)
					return err
				}
				delete(reapEvictedPods, k)
			}

			// Delete if the checker pod is Failed and there are more than 5 Failed checker pods of the same type which were created more recently
			if v.Status.Phase == "Failed" && oldCount > MaxPodsThreshold && failedCount > MaxPodsThreshold {
				err = deleteEvictedPod(client, v)
				if err != nil {
					log.Errorln("Failed to delete evicted pod:", k, err)
					return err
				}
				delete(reapEvictedPods, k)
			}
		}
	}
	return err
}

// getAllCheckerPods finds all evicted checker pods for a given khcheck
func getAllPodsWithCheckName(reapEvictedPods map[string]*v1.Pod, pod *v1.Pod) []*v1.Pod {

	var allCheckPods []*v1.Pod

	checkName := pod.Annotations["comcast.github.io/check-name"]
	//label := "kuberhealthy-check-name=" + checkName

	log.Infoln("Finding all evicted checker pods with checkName:", checkName)

	for _, v := range reapEvictedPods {
		if v.Labels["kuberhealthy-check-name"] == checkName {
			allCheckPods = append(allCheckPods, v)
		}
	}

	return allCheckPods
}

// deleteEvictedPods deletes pods in all namespaces that have the status: Evicted
func deleteEvictedPod(client *kubernetes.Clientset, pod *v1.Pod) error {

	fmt.Println("\tDeleting Pod: ", pod.Name, " in namespace: ", pod.Namespace)
	propagationForeground := metav1.DeletePropagationForeground
	options := &metav1.DeleteOptions{PropagationPolicy: &propagationForeground}
	err := client.CoreV1().Pods(pod.Namespace).Delete(pod.Name, options)
	if err != nil {
		return err
	}

	return nil
}
