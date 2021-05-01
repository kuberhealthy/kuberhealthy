package main

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v13 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	metav1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Use exponential backoff for retries
var exponentialBackoff = backoff.NewExponentialBackOff()

const maxElapsedTime = time.Minute

func init() {
	exponentialBackoff.MaxElapsedTime = maxElapsedTime
}

// getDSClient returns a daemonset client, useful for interacting with daemonsets
func getDSClient() v1.DaemonSetInterface {
	log.Debug("Creating Daemonset client.")
	return client.AppsV1().DaemonSets(checkNamespace)
}

// getPodClient returns a pod client, useful for interacting with pods
func getPodClient() metav1.PodInterface {
	log.Debug("Creating Pod client.")
	return client.CoreV1().Pods(checkNamespace)
}

// getNodeClient returns a node client, useful for interacting with nodes
func getNodeClient() metav1.NodeInterface {
	log.Debug("Creating Node client.")
	return client.CoreV1().Nodes()
}

func createDaemonset(daemonsetSpec *appsv1.DaemonSet) error {

	err := backoff.Retry(func() error {
		var err error
		_, err = getDSClient().Create(context.TODO(), daemonsetSpec, metav1.CreateOptions{})
		return err
	}, exponentialBackoff)
	if err != nil {
		log.Errorln("Failed to create daemonset. Error:", err)
		return err
	}

	return err
}

func listDaemonsets(more string) (*appsv1.DaemonSetList, error) {

	var dsList *appsv1.DaemonSetList
	err := backoff.Retry(func() error {
		var err error
		dsList, err = getDSClient().List(context.TODO(), metav1.ListOptions{
			Continue: more,
		})
		return err
	}, exponentialBackoff)
	if err != nil {
		log.Errorln("Failed to list daemonsets. Error:", err)
		return dsList, err
	}

	return dsList, err
}

func deleteDaemonset(dsName string) error {

	err := backoff.Retry(func() error {
		var err error
		err = getDSClient().Delete(context.TODO(), dsName, metav1.DeleteOptions{})
		return err
	}, exponentialBackoff)
	if err != nil {
		log.Errorln("Failed to delete daemonset. Error:", err)
		return err
	}

	return err
}

func listPods() (*v13.PodList, error) {

	var podList *v13.PodList
	err := backoff.Retry(func() error {
		var err error
		podList, err = getPodClient().List(context.TODO(), metav1.ListOptions{
			LabelSelector: "kh-app=" + daemonSetName + ",source=kuberhealthy,khcheck=daemonset",
		})
		return err
	}, exponentialBackoff)
	if err != nil {
		log.Errorln("Failed to list daemonset pods. Error:", err)
		return podList, err
	}

	return podList, err
}

func deletePods(dsName string) error {

	err := backoff.Retry(func() error {
		var err error
		err = getPodClient().DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: "kh-app=" + dsName + ",source=kuberhealthy,khcheck=daemonset",
		})
		return err
	}, exponentialBackoff)
	if err != nil {
		log.Errorln("Failed to delete daemonset pods. Error:", err)
		return err
	}

	return err
}

func listNodes() (*v13.NodeList, error) {

	var nodeList *v13.NodeList
	err := backoff.Retry(func() error {
		var err error
		nodeList, err = getNodeClient().List(context.TODO(), metav1.ListOptions{})
		return err
	}, exponentialBackoff)
	if err != nil {
		log.Errorln("Failed to list nodes. Error:", err)
		return nodeList, err
	}

	return nodeList, err
}
