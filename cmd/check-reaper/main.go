package main

import (
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
)

var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var ReapCheckerPods map[string]v1.Pod
var MaxPodsThreshold = 4
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

	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	podList, err := listCheckerPods(client, Namespace)
	if err != nil {
		log.Fatalln("Failed to list and delete old checker pods", err)
	}

	if len(podList) == 0 {
		log.Infoln("No pods found.")
		return
	}

	err = deleteFilteredCheckerPods(client, podList)
	if err != nil {
		log.Fatalln("Error found while deleting old pods:", err)
	}

	log.Infoln("Finished reaping checker pods.")
}

// listCheckerPods returns a list of pods with the khcheck name label
func listCheckerPods(client *kubernetes.Clientset, namespace string) (map[string]v1.Pod, error) {
	log.Infoln("Listing checker pods")

	ReapCheckerPods = make(map[string]v1.Pod)

	pods, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "kuberhealthy-check-name"})
	if err != nil {
		log.Errorln("Failed to list checker pods")
		return ReapCheckerPods, err
	}

	log.Infoln("Found:", len(pods.Items), "checker pods")

	for _, p := range pods.Items {
		if p.Status.Phase == "Succeeded" || p.Status.Phase == "Failed" {
			//log.Infoln("Checker pod: ", p.Name, "found in namespace: ", p.Namespace)
			ReapCheckerPods[p.Name] = p
		}
	}

	return ReapCheckerPods, err
}

// deleteFilteredCheckerPods goes through map of all checker pods and deletes older checker pods
func deleteFilteredCheckerPods(client *kubernetes.Clientset, reapCheckerPods map[string]v1.Pod) error {

	var err error

	for k, v := range reapCheckerPods {

		// Delete pods older than 5 hours and is in status Succeeded
		if time.Now().Sub(v.CreationTimestamp.Time).Hours() > 5 && v.Status.Phase == "Succeeded" {
			log.Infoln("Found pod older than 5 hours in status `Succeeded`. Deleting pod:", k)

			err = deletePod(client, v)
			if err != nil {
				log.Errorln("Failed to delete pod:", k, err)
				continue
			}
			delete(reapCheckerPods, k)
		}

		// Delete failed pods (status Failed) older than 5 days (120 hours)
		if time.Now().Sub(v.CreationTimestamp.Time).Hours() > 120 && v.Status.Phase == "Failed" {
			log.Infoln("Found pod older than 5 days in status `Failed`. Deleting pod:", k)

			err = deletePod(client, v)
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
				if v.CreationTimestamp.Time.Before(p.CreationTimestamp.Time) && p.Status.Phase != "Succeeded" && v.Namespace == p.Namespace {
					failOldCount++
				}
				if p.Status.Phase != "Succeeded" && v.Namespace == p.Namespace {
					failCount++
				}
				if v.CreationTimestamp.Time.Before(p.CreationTimestamp.Time) && p.Status.Phase == "Succeeded" && v.Namespace == p.Namespace {
					successOldCount++
				}
				if p.Status.Phase == "Succeeded" && v.Namespace == p.Namespace {
					successCount++
				}
			}

			// Delete if there are more than 5 checker pods with the same name in status Succeeded that were created more recently
			if  v.Status.Phase == "Succeeded" && successOldCount > MaxPodsThreshold && successCount > MaxPodsThreshold {
				log.Infoln("Found more than 5 checker pods with the same name in status `Succeeded` that were created more recently. Deleting pod:", k)

				err = deletePod(client, v)
				if err != nil {
					log.Errorln("Failed to delete pod:", k, err)
					continue
				}
				delete(reapCheckerPods, k)
			}

			// Delete if the checker pod is Failed and there are more than 5 Failed checker pods of the same type which were created more recently
			if v.Status.Phase == "Failed" && failOldCount > MaxPodsThreshold && failCount > MaxPodsThreshold {
				log.Infoln("Found more than 5 `Failed` checker pods of the same type which were created more recently. Deleting pod:", k)

				err = deletePod(client, v)
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
func deletePod(client *kubernetes.Clientset, pod v1.Pod) error {

	log.Infoln("Deleting Pod: ", pod.Name, " in namespace: ", pod.Namespace)
	propagationForeground := metav1.DeletePropagationForeground
	options := &metav1.DeleteOptions{PropagationPolicy: &propagationForeground}
	return client.CoreV1().Pods(pod.Namespace).Delete(pod.Name, options)
}
