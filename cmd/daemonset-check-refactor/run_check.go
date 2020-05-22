package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/util"
)

func (dsc *Checker) runDaemonsetCheck() {

	log.Infoln("Deleting any rogue daemonsets or daemonset pods before deploying daemonset")

	err := cleanupOrphans()
	if err != nil {
		errorMessage := "Error cleaning up rogue daemonsets or daemonset pods: " + err.Error()
		log.Errorln(errorMessage)
		reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + errorMessage})
	}

	// init a timeout for this whole deletion of daemonsets
	log.Infoln("Timeout set:", checkPodTimeout.String(), "for pre-check cleanup")
	checkPodTimeoutChan := time.After(checkPodTimeout)
	cleanUpCtx, cleanUpCtxCancel := context.WithCancel(context.Background())

	// waiting for all daemonsets to be gone...
	log.Infoln("Waiting for all rogue daemonsets or daemonset pods to clean up")

	select {
	case err = <-waitForAllDaemonsetsToClear(cleanUpCtx):
		if err != nil {
			cleanUpCtxCancel() // cancel the watch context, we have timed out
			errorMessage := "Error waiting for rogue daemonset or daemonset pods to clean up: " + err.Error()
			log.Errorln(errorMessage)
			reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + errorMessage})
		}
		log.Infoln("No rogue daemonsets or daemonset pods exist")
	case <-cleanUpCtx.Done():
		// If there is a cancellation interrupt signal.
		log.Infoln("Canceling cleanup and shutting down from interrupt.")
		reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: failed to perform pre-check cleanup within timeout"})
		return
	case <-checkPodTimeoutChan:
		log.Infoln("Timed out: reached time out for pre-check cleanup")
		cleanUpCtxCancel() // cancel the watch context, we have timed out
		errorMessage := "failed to perform pre-check cleanup within timeout: " + checkPodTimeout.String()
		log.Errorln(errorMessage)
		reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + errorMessage})
	}

	log.Infoln("Running daemonset check")

	err = dsc.Run(client)
	if err != nil {
		log.Errorln("Error running check:", err)
		os.Exit(1)
	}
	log.Debugln("Done running daemonset check")

}

// cleanupOrphans cleans up orphaned pods and daemonsets, if they exist
func cleanupOrphans() error {

	// first, clean up daemonsets
	err := cleanupOrphanedDaemonsets()
	if err != nil {
		return err
	}

	// we must also remove pods directly because they sometimes are able to exist
	// even when their creating daemonset has been removed.
	err = cleanupOrphanedPods()
	return err

}

// Run implements the entrypoint for check execution
func (dsc *Checker) Run(client *kubernetes.Clientset) error {

	doneChan := make(chan error)

	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := dsc.doChecks()
		doneChan <- err
	}(doneChan)

	var err error
	// wait for either a timeout or job completion
	select {
	case <-time.After(checkPodTimeout):
		// The check has reached its check pod timeout.
		ctxCancel() // cancel context
		errorMessage := "Failed to complete checks for " + dsc.Name() + " in time!  Next run came up but check was still running."
		//log.Errorln(dsc.Name(), errorMessage)
		err = checkclient.ReportFailure([]string{errorMessage})
		if err != nil {
			log.Println("Error reporting failure to Kuberhealthy servers:", err)
			return err
		}
		log.Println("Successfully reported failure to Kuberhealthy servers")
	case <-time.After(checkTimeLimit):
		// The check has timed out after its specified timeout period
		dsc.cancelFunc() // cancel context
		errorMessage := "Failed to complete checks for " + dsc.Name() + " in time!  Timeout was reached."
		//log.Errorln(dsc.Name(), errorMessage)
		err = checkclient.ReportFailure([]string{errorMessage})
		if err != nil {
			log.Println("Error reporting failure to Kuberhealthy servers:", err)
			return err
		}
		log.Println("Successfully reported failure to Kuberhealthy servers")
	case err := <-doneChan:
		dsc.cancelFunc()
		if err != nil {
			log.Println("Check failed with error:", err)
			err = checkclient.ReportFailure([]string{err.Error()})
			if err != nil {
				log.Println("Failed to report error to Kuberhealthy servers:", err)
				return err
			}
			log.Println("Successfully reported error to Kuberhealthy servers")
			return nil
		}
		err = checkclient.ReportSuccess()
		if err != nil {
			log.Println("Error reporting success to Kuberhealthy servers:", err)
			return err
		}
		log.Println("Successfully reported success to Kuberhealthy servers")
	}

	return nil
}

// doChecks actually runs checking procedures
func (dsc *Checker) doChecks() error {

	// deploy the daemonset
	err := dsc.doDeploy()
	if err != nil {
		return err
	}

	// clean up the daemonset.  Does not return until removed completely or
	// an error occurs
	err = dsc.doRemove()
	if err != nil {
		return err
	}

	// fire off an orphan cleanup in the background on each check run
	go cleanupOrphans()

	return nil
}

// doDeploy actually deploys the DS into the cluster
func (dsc *Checker) doDeploy() error {

	// create DS
	dsc.DaemonSetDeployed = true
	err := dsc.deploy()
	if err != nil {
		log.Error("Something went wrong with daemonset deployment, cleaning things up...", err)
		err2 := dsc.doRemove()
		if err2 != nil {
			log.Error("Something went wrong when cleaning up the daemonset after the daemonset deployment error:", err2)
		}
		return err
	}

	select {
	case err = <-dsc.waitForPodsToComeOnline():
		if err != nil {
			ctxCancel() // cancel the watch context, we have timed out
			log.Errorln("Error waiting for pods to come online up:", err)
		}
		log.Infoln("No rogue daemonsets or daemonset pods exist")
	}
	return err
}

// Deploy creates a daemon set
func (dsc *Checker) deploy() error {
	//Generate the spec for the DS that we are about to deploy
	dsc.generateDaemonSetSpec()
	//Generate DS client and create the set with the template we just generated
	daemonSetClient := getDSClient()
	_, err := daemonSetClient.Create(dsc.DaemonSet)
	if err != nil {
		log.Error("Failed to create daemon set:", err)
	}
	dsc.DaemonSetDeployed = true
	return err
}

// generateDaemonSetSpec generates a daemon set spec to deploy into the cluster
func (dsc *Checker) generateDaemonSetSpec() {

	checkRunTime := strconv.Itoa(int(now.Unix()))
	terminationGracePeriod := int64(1)

	// Set the runAsUser
	runAsUser := defaultUser
	currentUser, err := util.GetCurrentUser(defaultUser)
	if err != nil {
		log.Infoln("Unable to get the current user id %v", err)
	}
	log.Infoln("runAsUser will be set to %v", currentUser)
	runAsUser = currentUser

	// if a list of tolerations wasnt passed in, default to tolerating all taints
	if len(dsc.Tolerations) == 0 {
		// find all the taints in the cluster and create a toleration for each
		dsc.Tolerations, err = findAllUniqueTolerations(client)
		if err != nil {
			log.Warningln("Unable to generate list of pod scheduling tolerations", err)
		}
	}

	// create the DS object
	log.Infoln("Generating daemon set kubernetes spec.")
	dsc.DaemonSet = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: dsc.DaemonSetName,
			Labels: map[string]string{
				"app":              dsc.DaemonSetName,
				"source":           "kuberhealthy",
				"khcheck":          "daemonset",
				"creatingInstance": dsc.hostname,
				"checkRunTime":     checkRunTime,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			MinReadySeconds: 2,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":              dsc.DaemonSetName,
					"source":           "kuberhealthy",
					"khcheck":          "daemonset",
					"creatingInstance": dsc.hostname,
					"checkRunTime":     checkRunTime,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":              dsc.DaemonSetName,
						"source":           "kuberhealthy",
						"khcheck":          "daemonset",
						"creatingInstance": dsc.hostname,
						"checkRunTime":     checkRunTime,
					},
					Name: dsc.DaemonSetName,
					Annotations: map[string]string{
						"cluster-autoscaler.kubernetes.io/safe-to-evict": "true",
					},
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Tolerations:                   []apiv1.Toleration{},
					Containers: []apiv1.Container{
						{
							Name:  "sleep",
							Image: dsc.PauseContainerImage,
							SecurityContext: &apiv1.SecurityContext{
								RunAsUser: &runAsUser,
							},
							Resources: apiv1.ResourceRequirements{
								Requests: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse("0"),
									apiv1.ResourceMemory: resource.MustParse("0"),
								},
							},
						},
					},
				},
			},
		},
	}

	// Add our generated list of tolerations or any the user input via flag
	log.Infoln("Deploying daemon set with tolerations: ", dsc.DaemonSet.Spec.Template.Spec.Tolerations)
	dsc.DaemonSet.Spec.Template.Spec.Tolerations = append(dsc.DaemonSet.Spec.Template.Spec.Tolerations, dsc.Tolerations...)
}

// findAllUniqueTolerations returns a list of all taints present on any node group in the cluster
// this is exportable because of a chicken/egg.  We need to determine the taints before
// we construct the testDS in New() and pass them into New()
func findAllUniqueTolerations(client *kubernetes.Clientset) ([]apiv1.Toleration, error) {

	var uniqueTolerations []apiv1.Toleration

	// get a list of all the nodes in the cluster
	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return uniqueTolerations, err
	}
	log.Infoln("Searching for unique taints on the cluster.")
	// this keeps track of the unique taint values
	keys := make(map[string]bool)
	// get a list of all taints
	for _, n := range nodes.Items {
		for _, t := range n.Spec.Taints {
			// only add unique entries to the slice
			if _, value := keys[t.Value]; !value {
				keys[t.Value] = true
				// Add the taints to the list as tolerations
				// daemonset.spec.template.spec.tolerations
				uniqueTolerations = append(uniqueTolerations, apiv1.Toleration{Key: t.Key, Value: t.Value, Effect: t.Effect})
			}
		}
	}
	log.Infoln("Found taints to tolerate:", uniqueTolerations)
	return uniqueTolerations, nil
}

// doRemove remotes the daemonset from the cluster
func (dsc *Checker) doRemove() error {
	// delete ds
	err := deleteDS(dsc.DaemonSetName)
	if err != nil {
		return err
	}

	// wait for daemonset to be removed
	err = dsc.waitForDSRemoval()
	if err != nil {
		return err
	}

	// wait for ds pods to be deleted
	err = dsc.waitForPodRemoval()
	if err != nil {
		return err
	}
	dsc.DaemonSetDeployed = true
	return err
}

// waitForPodsToComeOnline blocks until all pods of the daemonset are deployed and online
func (dsc *Checker) waitForPodsToComeOnline() chan error {

	log.Infoln("Waiting for all ds pods to come online")

	// make the output channel we will return and close it whenever we are done
	outChan := make(chan error, 2)

	// counter for DS status check below
	var counter int
	var nodesMissingDSPod []string

	log.Infoln("Timeout set:", checkPodTimeout.String(), "for all daemonset pods to come online")
	checkPodTimeoutChan := time.After(checkPodTimeout)

	//select {
	//case <-checkPodTimeoutChan:
	//	log.Infoln("Timed out: reached time out for all daemonset pods to come online")
	//	ctxCancel() // cancel the watch context, we have timed out
	//	errorMessage := "Failed to see all daemonset pods come up within check pod timeout: " + checkPodTimeout.String()

	go func() {
		for {
			select {
			case <- ctx.Done():
				errorMessage := "DaemonsetChecker: Node(s) which were unable to schedule before context was cancelled: " + formatNodes(nodesMissingDSPod)
				log.Errorln(errorMessage)
				outChan <- errors.New(errorMessage)
				return
			case <- checkPodTimeoutChan:
				log.Infoln("")
				errorMessage := "DaemonsetChecker: Node(s) were unable to schedule daemonset pod before check pod timeout: " + formatNodes(nodesMissingDSPod)
				log.Errorln(errorMessage)
				outChan <- errors.New(errorMessage)
			default:
			}

			time.Sleep(time.Second)

			// if we need to shut down, stop waiting entirely
			if dsc.shuttingDown {
				errorMessage := "DaemonsetChecker: Nodes which were unable to schedule before shutdown signal was received:" + formatNodes(nodesMissingDSPod)
				log.Errorln(errorMessage)
				outChan <- errors.New(errorMessage)
				return
			}

			// check the number of nodes in the cluster.  Make sure we have that many
			// pods scheduled.

			// find nodes missing pods from this daemonset
			nodesMissingDSPod, err := dsc.getNodesMissingDSPod()
			if err != nil {
				log.Warningln("DaemonsetChecker: Error determining which node was unschedulable. Retrying.", err)
				continue
			}

			// We want to ensure all the DS pods are up and healthy for at least 5 seconds
			// before moving on. This is to help verify that the DS is _actually_ healthy
			// and to mitigate possible race conditions arising from deleting pods that
			// were _just_ created

			// The DS must not have any nodes missing pods for five iterations in a row
			readySeconds := 5
			if len(nodesMissingDSPod) <= 0 {
				counter++
				log.Infoln("DaemonsetChecker: All daemonset pods have been ready for", counter, "/", readySeconds, "seconds.")
				if counter >= readySeconds {
					log.Infoln("DaemonsetChecker: Daemonset " + dsc.DaemonSetName + " done deploying pods.")
					outChan <- nil
					return
				}
				continue
			}
			// else if we've started counting up but there is a DS pod that went unready
			// reset the counter
			if counter > 0 {
				log.Infoln("DaemonsetChecker: Daemonset "+dsc.DaemonSetName+" was ready for", counter, "out of,", readySeconds, "seconds but has left the ready state. Restarting", readySeconds, "second timer.")
				counter = 0
			}
			// If the counter isnt iterating up or being reset, we are still waiting for pods to come online
			log.Infoln("DaemonsetChecker: Daemonset check waiting for", len(nodesMissingDSPod), "pods to come up on nodes", nodesMissingDSPod)
		}
	}()
	return outChan
}

func formatNodes(nodeList []string) string {
	if len(nodeList) > 0 {
		return strings.Join(nodeList,", ")
	} else {
		return ""
	}
}

// getNodesMissingDSPod gets a list of nodes that do not have a DS pod running on them
func (dsc *Checker) getNodesMissingDSPod() ([]string, error) {

	// nodesMissingDSPods holds the final list of nodes missing pods
	var nodesMissingDSPods []string

	// get a list of all the nodes in the cluster
	nodeClient := getNodeClient()
	nodes, err := nodeClient.List(metav1.ListOptions{})
	if err != nil {
		return nodesMissingDSPods, err
	}

	// get a list of DS pods
	podClient := getPodClient()
	pods, err := podClient.List(metav1.ListOptions{
		LabelSelector: "app=" + dsc.DaemonSetName + ",source=kuberhealthy,khcheck=daemonset",
	})
	if err != nil {
		return nodesMissingDSPods, err
	}

	// populate a node status map. default status is "false", meaning there is
	// not a pod deployed to that node.  We are only adding nodes that tolerate
	// our list of dsc.Tolerations
	nodeStatuses := make(map[string]bool)
	for _, n := range nodes.Items {
		if taintsAreTolerated(n.Spec.Taints, dsc.Tolerations) {
			nodeStatuses[n.Name] = false
		}
	}

	// Look over all daemonset pods.  Mark any hosts that host one of the pods
	// as "true" in the nodeStatuses map, indicating that a daemonset pod is
	// deployed there.
	//Additionally, only look on nodes with taints that we tolerate
	for _, pod := range pods.Items {
		// the pod should be ready
		if pod.Status.Phase != "Running" {
			continue
		}
		for _, node := range nodes.Items {
			for _, nodeip := range node.Status.Addresses {
				// We are looking for the Internal IP and it needs to match the host IP
				if nodeip.Type != "InternalIP" || nodeip.Address != pod.Status.HostIP {
					continue
				}
				if taintsAreTolerated(node.Spec.Taints, dsc.Tolerations) {
					nodeStatuses[node.Name] = true
					break
				}
			}
		}
	}

	// pick out all the nodes without daemonset pods on them and
	// add them to the final results
	for nodeName, hasDS := range nodeStatuses {
		if !hasDS {
			nodesMissingDSPods = append(nodesMissingDSPods, nodeName)
		}
	}

	return nodesMissingDSPods, nil
}

// taintsAreTolerated iterates through all taints and tolerations passed in
// and checks that all taints are tolerated by the supplied tolerations
func taintsAreTolerated(taints []apiv1.Taint, tolerations []apiv1.Toleration) bool {
	for _, taint := range taints {
		var taintIsTolerated bool
		for _, toleration := range tolerations {
			if taint.Key == toleration.Key && taint.Value == toleration.Value {
				taintIsTolerated = true
				break
			}
		}
		// if none of the tolerations match the taint, it is not tolerated
		if !taintIsTolerated {
			return false
		}
	}
	// if all taints have a matching toleration, return true
	return true
}

// cleanupOrphanedDaemonsets cleans up daemonsets that should not exist based on their
// creatingInstance label and ensures they are not daemonsets from an older run
func cleanupOrphanedDaemonsets() error {

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

// cleanupOrphanedPods cleans up daemonset pods that shouldn't exist because their
// creating instance is gone and ensures thay are not pods from an older run.
// Sometimes removing daemonsets isnt enough to clean up orphaned pods.
func cleanupOrphanedPods() error {
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


// getAllDaemonsets fetches all daemonsets created by the daemonset khcheck
func getAllDaemonsets() ([]appsv1.DaemonSet, error) {

	var allDS []appsv1.DaemonSet
	var cont string
	var err error

	// fetch the ds objects created by kuberhealthy
	for {
		var dsList *appsv1.DaemonSetList
		dsClient := getDSClient()
		dsList, err = dsClient.List(metav1.ListOptions{
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

// getAllPods fetches all pods in the namespace, for all instances of kuberhealthy
// based on a source=kuberhealthy label.
func getAllPods() ([]apiv1.Pod, error) {

	var allPods []apiv1.Pod
	var cont string

	// fetch the pod objects created by kuberhealthy
	for {
		var podList *apiv1.PodList
		podClient := getPodClient()
		podList, err := podClient.List(metav1.ListOptions{
			LabelSelector: "source=kuberhealthy,khcheck=daemonset",
		})
		if err != nil {
			log.Warningln("Unable to get all pods:", err)
		}
		cont = podList.Continue

		// pick the items out and add them to our end results
		for _, p := range podList.Items {
			allPods = append(allPods, p)
		}

		// while continue is set, keep fetching items
		if len(cont) == 0 {
			break
		}
	}

	return allPods, nil
}

// checkIfPodExists fetches a specific kuberhealthy pod by name to ensure
// it exists.
func checkIfPodExists(podName string) bool {
	podClient := getPodClient()
	_, err := podClient.Get(podName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}

// checkIfDSExists fetches a specific kuberhealthy ds by name to ensure
// it exists.
func checkIfDSExists(dsName string) bool {
	dsClient := getDSClient()
	_, err := dsClient.Get(dsName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}

// waitForAllDaemonsetsToClear
func waitForAllDaemonsetsToClear(ctx context.Context) chan error {

	// make the output channel we will return and close it whenever we are done
	outChan := make(chan error, 2)

	go func() {
		// watch events and return when the pod is in state running
		for {

			// wait between requests
			time.Sleep(time.Second * 5)

			// if the context is canceled, we stop
			select {
			case <- ctx.Done():
				outChan <- errors.New("Waiting for daemonset to clear was aborted by context cancellation")
				return
			default:
			}

			// fetch the pod by name
			dsList, err := getAllDaemonsets()
			if err != nil {
				outChan <- err
				return
			}
			if len(dsList) == 0 {
				log.Info("All daemonsets cleared")
				outChan <- nil
				return
			}
		}
	}()

	return outChan
}

