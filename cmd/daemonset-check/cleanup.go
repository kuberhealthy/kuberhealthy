package main

import (
	"context"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// cleanUp triggers check clean up and waits for all rogue daemonsets to clear
func cleanUp(ctx context.Context) error {

	log.Debugln("Allowing clean up", shutdownGracePeriod, "to finish.")

	// Clean up daemonsets and daemonset pods
	// deleteDS not only issues a delete on the rogue daemonset but also on the rogue daemonset pods
	log.Infoln("Cleaning up daemonsets and daemonset pods")

	daemonSets, err := getAllDaemonsets(ctx)
	if err != nil {
		return err
	}

	// If any rogue daemonsets are found, proceed to remove them.
	if len(daemonSets) > 0 {
		for _, ds := range daemonSets {
			log.Infoln("Removing rogue daemonset:", ds.Name)
			err := remove(ctx, ds.Name)
			if err != nil {
				return err
			}
		}
	}
	log.Infoln("Finished cleanup. No rogue daemonsets or daemonset pods exist")
	return nil
}

// getAllDaemonsets fetches all daemonsets created by the daemonset khcheck
func getAllDaemonsets(ctx context.Context) ([]appsv1.DaemonSet, error) {

	var allDS []appsv1.DaemonSet
	var cont string
	var err error

	// fetch the ds objects created by kuberhealthy
	for {
		var dsList *appsv1.DaemonSetList
		dsList, err = getDSClient().List(ctx, metav1.ListOptions{
			LabelSelector: "source=kuberhealthy,khcheck=daemonset",
		})
		if err != nil {
			errorMessage := "Error getting all daemonsets: " + err.Error()
			log.Errorln(errorMessage)
			return allDS, errors.New(errorMessage)
		}
		cont = dsList.Continue

		// pick the items out and add them to our end results
		for _, ds := range dsList.Items {
			allDS = append(allDS, ds)
		}

		// while continue is set, keep fetching items
		if len(cont) == 0 {
			break
		}
	}

	return allDS, nil
}
