package main

import (
	"context"
	"strconv"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// cleanUp triggers check clean up and waits for all rogue daemonsets to clear
func cleanUp(ctx context.Context) error {

	log.Debugln("Allowing clean up", checkTimeLimit, "to finish.")
	err := runCheckCleanUp()
	if err != nil {
		return err
	}

	// waiting for all daemonSets to be gone...
	log.Infoln("Waiting for all daemonSets or daemonset pods to clean up")

	doneChan := make(chan error, 10)
	go func() {
		log.Debugln("Worker: waitForAllDaemonsetsToClear started")
		doneChan <- waitForAllDaemonsetsToClear(ctx)
	}()

	// wait for either daemonsets to clear or the timeout
	select {
	case err = <-doneChan:
		log.Infoln("Finished cleanup. No rogue daemonsets or daemonset pods exist")
		return err
	case <-ctx.Done():
	case <-time.After(checkTimeLimit):
		unClearedDSList := getUnClearedDSList(daemonsetList)
		return errors.New("Reached check pod timeout: " + checkTimeLimit.String() + " waiting for all daemonsets to clear. " + "Daemonset that failed to clear: " + unClearedDSList)
	}

	return nil
}

// runCheckCleanUp cleans up rogue pods and daemonsets, if they exist
func runCheckCleanUp() error {

	// first, clean up daemonsets
	err := cleanUpDaemonsets()
	if err != nil {
		return err
	}

	// we must also remove pods directly because they sometimes are able to exist
	// even when their creating daemonset has been removed.
	err = cleanUpPods()
	return err

}

// cleanUpPods cleans up daemonset pods that shouldn't exist because their
// creating instance is gone and ensures thay are not pods from an older run.
// Sometimes removing daemonsets isnt enough to clean up orphaned pods.
func cleanUpPods() error {

	log.Infoln("Cleaning up daemonset pods")

	pods, err := getAllPods()
	if err != nil {
		log.Errorln("Error fetching pods:", err)
		return err
	}

	// loop on all the daemonsets and ensure that daemonset's creating pod exists and that the pods are not from an older run
	// if the creating pod does not exist, then we delete the daemonset.
	for _, p := range pods {
		log.Infoln("Checking if pod is orphaned:", p.Name, "creatingInstance:", p.Labels["creatingInstance"], "checkRunTime:", now.Unix())

		// fetch the creatingInstance label
		creatingDSInstance := p.Labels["app"]

		// if there isnt a creatingInstance label, we assume its an old generation and remove it.
		if len(creatingDSInstance) == 0 {
			log.Warningln("Unable to find app label on pod", p.Name, "assuming orphaned and removing!")
			err := deletePod(p.Name)
			if err != nil {
				log.Warningln("error when removing orphaned pod due to missing label", p.Name+": ", err)
			}
			continue
		}

		// check if the creatingInstance exists
		exists := checkIfDSExists(creatingDSInstance)

		// if the owning kuberhealthy pod of the DS does not exist, then we delete the daemonset
		if !exists {
			log.Infoln("Removing orphaned pod", p.Name, "because kuberhealthy ds", creatingDSInstance, "does not exist")
			err := deletePod(p.Name)
			if err != nil {
				log.Warningln("error when removing orphaned pod", p.Name+": ", err)
				return err
			}
		}

		// Check that the pod isn't from an older run
		podCheckRunTime, err := strconv.ParseInt(p.Labels["checkRunTime"], 10, 64)
		if err != nil {
			log.Errorln("Error converting pod checkRunTime:", podCheckRunTime, "label to int:", err)
		}

		if podCheckRunTime < now.Unix() {
			log.Warningln("Pod:", p.Name, "has an older checkRunTime than the current daemonset running. This is a rogue pod, removing now.")
			err := deletePod(p.Name)
			if err != nil {
				log.Warningln("error when removing rogue pod:", p.Name+": ", err)
			}
			continue
		}
	}
	return nil
}

// cleanUpDaemonsets cleans up daemonsets that should not exist based on their
// creatingInstance label and ensures they are not daemonsets from an older run
func cleanUpDaemonsets() error {

	log.Infoln("Cleaning up daemonsets")

	daemonSets, err := getAllDaemonsets()
	if err != nil {
		return err
	}

	// loop on all the daemonsets and ensure that daemonset's creating pod exists and that the daemonsets are not from an older run.
	// if the creating pod does not exist, then we delete the daemonset.
	for _, ds := range daemonSets {
		log.Infoln("Checking if daemonset is orphaned:", ds.Name, "creatingInstance:", ds.Labels["creatingInstance"], "checkRunTime:", now.Unix())

		// fetch the creatingInstance label
		creatingInstance := ds.Labels["creatingInstance"]

		// if there isn't a creatingInstance label, we assume its an old generation and remove it.
		if len(creatingInstance) == 0 {
			log.Warningln("Unable to find hostname with creatingInstance label on ds", ds.Name, "assuming orphaned and removing!")
			err := deleteDS(ds.Name)
			if err != nil {
				log.Warningln("error when removing orphaned daemonset due to missing label", ds.Name+": ", err)
				return err
			}
			continue
		}

		// check if the creatingInstance exists
		exists := checkIfPodExists(creatingInstance)

		// if the owning daemonset checker pod does not exist, then we delete the daemonset
		if !exists {
			log.Infoln("Removing orphaned daemonset", ds.Name, "because creating kuberhealthy damonset checker instance", creatingInstance, "does not exist")
			err := deleteDS(ds.Name)
			if err != nil {
				log.Errorln("error when removing orphaned daemonset", ds.Name+": ", err)
				return err
			}
		}

		// Check that the daemonset isn't from an older run
		dsCheckRunTime, err := strconv.ParseInt(ds.Labels["checkRunTime"], 10, 64)
		if err != nil {
			log.Errorln("Error converting ds checkRunTime:", dsCheckRunTime, "label to int:", err)
		}

		if dsCheckRunTime < now.Unix() {
			log.Warningln("Daemonset:", ds.Name, "has an older checkRunTime than the current daemonset running. This is a rogue daemonset, removing now.")
			err := deleteDS(ds.Name)
			if err != nil {
				log.Errorln("error when removing rogue daemonset:", ds.Name+": ", err)
			}
			continue
		}
	}
	return nil
}
