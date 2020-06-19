package main

import (
	"time"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v13 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"

	"gopkg.in/matryer/try.v1"
)

const kubeAPIAttempts = 3
const timeBetweenAttempts = time.Second

// getDSClient returns a daemonset client, useful for interacting with daemonsets
func getDSClient() v1.DaemonSetInterface {
	log.Debug("Creating Daemonset client.")
	return client.AppsV1().DaemonSets(checkNamespace)
}

// getPodClient returns a pod client, useful for interacting with pods
func getPodClient() v12.PodInterface {
	log.Debug("Creating Pod client.")
	return client.CoreV1().Pods(checkNamespace)
}

// getNodeClient returns a node client, useful for interacting with nodes
func getNodeClient() v12.NodeInterface {
	log.Debug("Creating Node client.")
	return client.CoreV1().Nodes()
}

func createDaemonset(daemonsetSpec *appsv1.DaemonSet) error {

	err := try.Do(func(attempt int) (bool, error) {
		var err error
		_, err = getDSClient().Create(daemonsetSpec)
		if err != nil {
			time.Sleep(timeBetweenAttempts) // wait in between requests
		}
		return attempt < kubeAPIAttempts, err
	})
	if err != nil {
		log.Errorln("Failed to create daemonset after", kubeAPIAttempts, "attempts. Error:", err)
		return err
	}
	return err
}

func listDaemonsets(more string) (*appsv1.DaemonSetList, error) {

	var dsList *appsv1.DaemonSetList
	err := try.Do(func(attempt int) (bool, error) {
		var err error
		dsList, err = getDSClient().List(metav1.ListOptions{
			Continue: more,
		})
		if err != nil {
			time.Sleep(timeBetweenAttempts) // wait in between requests
		}
		return attempt < kubeAPIAttempts, err
	})
	if err != nil {
		log.Errorln("Failed to list daemonsets after", kubeAPIAttempts, "attempts. Error:", err)
		return dsList, err
	}
	return dsList, err
}

func deleteDaemonset(dsName string) error {

	err := try.Do(func(attempt int) (bool, error) {
		var err error
		err = getDSClient().Delete(dsName, &metav1.DeleteOptions{})
		if err != nil {
			time.Sleep(timeBetweenAttempts) // wait in between requests
		}
		return attempt < kubeAPIAttempts, err
	})
	if err != nil {
		log.Errorln("Failed to delete daemonset after", kubeAPIAttempts, "attempts. Error:", err)
		return err
	}
	return err
}

func listPods() (*v13.PodList, error) {

	var podList *v13.PodList
	err := try.Do(func(attempt int) (bool, error) {
		var err error
		podList, err = getPodClient().List(metav1.ListOptions{
			LabelSelector: "app=" + daemonSetName + ",source=kuberhealthy,khcheck=daemonset",
		})
		if err != nil {
			time.Sleep(timeBetweenAttempts) // wait in between requests
		}
		return attempt < kubeAPIAttempts, err
	})
	if err != nil {
		log.Errorln("Failed to list daemonset pods after", kubeAPIAttempts, "attempts. Error:", err)
		return podList, err
	}
	return podList, err
}

func deletePods(dsName string) error {

	err := try.Do(func(attempt int) (bool, error) {
		var err error
		err = getPodClient().DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: "app=" + dsName + ",source=kuberhealthy,khcheck=daemonset",
		})
		if err != nil {
			time.Sleep(timeBetweenAttempts) // wait in between requests
		}
		return attempt < kubeAPIAttempts, err
	})
	if err != nil {
		log.Errorln("Failed to delete daemonset pods after", kubeAPIAttempts, "attempts. Error:", err)
		return err
	}
	return err
}

func listNodes() (*v13.NodeList, error){

	var nodeList *v13.NodeList
	err := try.Do(func(attempt int) (bool, error) {
		var err error
		nodeList, err = getNodeClient().List(metav1.ListOptions{})
		if err != nil {
			time.Sleep(timeBetweenAttempts) // wait in between requests
		}
		return attempt < kubeAPIAttempts, err
	})
	if err != nil {
		log.Errorln("Failed to list nodes after", kubeAPIAttempts, "attempts. Error:", err)
		return nodeList, err
	}
	return nodeList, err
}
