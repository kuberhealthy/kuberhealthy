package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/util"
)

var daemonSetList []appsv1.DaemonSet
var nodesMissingDSPod []string
var podRemovalList *apiv1.PodList

// runCheck runs pre-check cleanup and then the full daemonset check. If successful, reports OK to kuberhealthy.
func runCheck() {

	log.Infoln("Running pre-check cleanup. Deleting any rogue daemonsets or daemonset pods before running daemonset check.")
	err := cleanUp()
	if err != nil {
		log.Errorln("Error running pre-check cleanup:", err)
		os.Exit(1)
	}

	log.Infoln("Running daemonset check")

	err = runDaemonsetCheck(client)
	if err != nil {
		log.Errorln("Error running check:", err)
		os.Exit(1)
	}
	log.Infoln("Finished running checks, reporting OK to Kuberhealthy ")
	reportOKToKuberhealthy()
	log.Infoln("Done running daemonset check")
}

// cleanup triggers run check clean up and waits for all rogue daemonsets to clear
func cleanUp() error {

	log.Debugln("Allowing this clean up", checkPodTimeout, "to finish.")
	cleanUpCtx, cleanUpCtxCancel = context.WithTimeout(context.Background(), checkPodTimeout)

	err := runCheckCleanUp()
	if err != nil {
		errorMessage := "Error running cleanup: " + err.Error()
		log.Errorln(errorMessage)
		reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + errorMessage})
		return errors.New(errorMessage)
	}

	// waiting for all daemonsets to be gone...
	log.Infoln("Waiting for all daemonsets or daemonset pods to clean up")

	outChan := make(chan error, 10)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		log.Infoln("Worker: waitForAllDaemonsetsToClear started")
		defer close(outChan)
		outChan <- waitForAllDaemonsetsToClear()
		wg.Wait()
	}()

	select {
	case err = <-outChan:
		if err != nil {
			cleanUpCtxCancel() // cancel the watch context, we have timed out
			log.Errorln(err)
			reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + err.Error()})
			return err
		}
		log.Infoln("Successfully finished cleanup. No rogue daemonsets or daemonset pods exist")
	case <- time.After(checkPodTimeout):
		var errorMessage string
		if len(daemonSetList) > 0 {
			unClearedDSList := getUnClearedDSList(daemonSetList)
			errorMessage = "Reached check pod timeout: " + checkPodTimeout.String() + " waiting for all daemonsets to clear. " +
				"Daemonset that failed to clear: " + strings.Join(unClearedDSList, ", ")
		}
		errorMessage = "Reached check pod timeout: " + checkPodTimeout.String() + " waiting for all daemonsets to clear."
		log.Errorln(errorMessage)
		reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + errorMessage})
		return errors.New(errorMessage)
	case <-cleanUpCtx.Done():
		// If there is a cancellation interrupt signal.
		log.Infoln("Canceling cleanup and shutting down from interrupt.")
		return err
	}
	return err
}

// cleanupOrphans cleans up orphaned pods and daemonsets, if they exist
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

// Run implements the entrypoint for check execution
func runDaemonsetCheck(client *kubernetes.Clientset) error {

	// Deploy Daemonset
	err := doDeploy()
	if err != nil {
		return err
	}

	// Clean up the Daemonset. Does not return until removed completely or an error occurs
	err = doRemove()
	if err != nil {
		return err
	}

	log.Infoln("Running post-check cleanup. Deleting any rogue daemonsets or daemonset pods after finishing check.")
	go cleanUp()

	return nil
}

func doDeploy() error {
	log.Infoln("Deploying daemonset.")

	daemonSetDeployed = true
	err := deploy()
	if err != nil {
		log.Error("Something went wrong with daemonset deployment, cleaning things up...", err)
		err2 := doRemove()
		if err2 != nil {
			log.Error("Something went wrong when cleaning up the daemonset after the daemonset deployment error:", err2)
		}
		return err
	}

	outChan := make(chan error, 10)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer close(outChan)
		outChan <- waitForPodsToComeOnline()
		wg.Wait()
	}()

	select {
	case err = <-outChan:
		if err != nil {
			ctxCancel() // cancel the watch context, we have timed out
			log.Errorln("Error waiting for pods to come online up:", err)
			reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + err.Error()})
			return err
		}
		log.Infoln("Successfully deployed daemonset.")
	case <- time.After(checkPodTimeout):
		var errorMessage string
		if len(nodesMissingDSPod) > 0 {
			errorMessage = "Reached check pod timeout: " + checkPodTimeout.String() + " waiting for all pods to come online. " +
				"Node(s) missing daemonset pod: " + strings.Join(nodesMissingDSPod, ", ")
		}
		errorMessage = "Reached check pod timeout: " + checkPodTimeout.String() + " waiting for all pods to come online."
		log.Errorln(errorMessage)
		reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + errorMessage})
		return errors.New(errorMessage)
	case <-ctx.Done():
		// If there is a cancellation interrupt signal.
		log.Infoln("Canceling deploying daemonset and shutting down from interrupt.")
		return err
	}
	return err
}


// Deploy creates a daemonset
func deploy() error {
	//Generate the spec for the DS that we are about to deploy
	generateDaemonSetSpec()
	//Generate DS client and create the set with the template we just generated
	daemonSetClient := getDSClient()
	_, err := daemonSetClient.Create(DaemonSet)
	if err != nil {
		log.Error("Failed to create daemonset:", err)
	}
	daemonSetDeployed = true
	return err
}

// generateDaemonSetSpec generates a daemonset spec to deploy into the cluster
func generateDaemonSetSpec() {

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
	if len(tolerations) == 0 {
		// find all the taints in the cluster and create a toleration for each
		tolerations, err = findAllUniqueTolerations(client)
		if err != nil {
			log.Warningln("Unable to generate list of pod scheduling tolerations", err)
		}
	}

	// create the DS object
	log.Infoln("Generating daemonset kubernetes spec.")
	DaemonSet = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: daemonSetName,
			Labels: map[string]string{
				"app":              daemonSetName,
				"source":           "kuberhealthy",
				"khcheck":          "daemonset",
				"creatingInstance": hostName,
				"checkRunTime":     checkRunTime,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			MinReadySeconds: 2,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":              daemonSetName,
					"source":           "kuberhealthy",
					"khcheck":          "daemonset",
					"creatingInstance": hostName,
					"checkRunTime":     checkRunTime,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":              daemonSetName,
						"source":           "kuberhealthy",
						"khcheck":          "daemonset",
						"creatingInstance": hostName,
						"checkRunTime":     checkRunTime,
					},
					Name: daemonSetName,
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
							Image: dsPauseContainerImage,
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
	log.Infoln("Deploying daemonset with tolerations: ", DaemonSet.Spec.Template.Spec.Tolerations)
	DaemonSet.Spec.Template.Spec.Tolerations = append(DaemonSet.Spec.Template.Spec.Tolerations, tolerations...)
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
func doRemove() error {
	log.Infoln("Removing daemonset.")

	err := deleteDS(daemonSetName)
	if err != nil {
		return err
	}

	outChanDS:= make(chan error, 10)
	outChanPods:= make(chan error, 10)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer close(outChanDS)
		outChanDS <- waitForDSRemoval()
		wg.Wait()
	}()

	select {
	case err = <-outChanDS:
		if err != nil {
			ctxCancel() // cancel the watch context, we have timed out
			log.Errorln("Error waiting for daemonset removal:", err)
			reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + err.Error()})
			return err
		}
		log.Infoln("Successfully removed daemonset.")
	case <- time.After(checkPodTimeout):
		errorMessage := "Reached check pod timeout: " + checkPodTimeout.String() + " waiting for daemonset removal."
		log.Errorln(errorMessage)
		reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + errorMessage})
		return errors.New(errorMessage)
	case <-ctx.Done():
		// If there is a cancellation interrupt signal.
		log.Infoln("Canceling removing daemonset and shutting down from interrupt.")
		return err
	}

	go func() {
		defer close(outChanPods)
		outChanPods <- waitForPodRemoval()
		wg.Wait()
	}()

	select {
	case err = <-outChanPods:
		if err != nil {
			ctxCancel() // cancel the watch context, we have timed out
			log.Errorln("Error waiting for daemonset pods removal:", err)
			reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + err.Error()})
			return err
		}
		log.Infoln("Successfully removed daemonset pods.")
	case <- time.After(checkPodTimeout):
		var errorMessage string
		if len(podRemovalList.Items) > 0 {
			unClearedDSPodsNodes := getDSPodsNodeList(podRemovalList)
			errorMessage = "Reached check pod timeout: " + checkPodTimeout.String() + " waiting for daemonset pods removal. " +
				"Node(s) failing to remove daemonset pod: " + strings.Join(unClearedDSPodsNodes, ", ")
		}
		errorMessage = "Reached check pod timeout: " + checkPodTimeout.String() + " waiting for daemonset pods removal."
		log.Errorln(errorMessage)
		reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + errorMessage})
		return errors.New(errorMessage)
	case <-ctx.Done():
		// If there is a cancellation interrupt signal.
		log.Infoln("Canceling removing daemonset pods and shutting down from interrupt.")
		return err
	}

	daemonSetDeployed = true
	return err
}

// waitForPodsToComeOnline blocks until all pods of the daemonset are deployed and online
func waitForPodsToComeOnline() error {

	log.Infoln("Waiting for all ds pods to come online")

	// counter for DS status check below
	var counter int

	// init a timeout for this whole deletion of daemonsets
	log.Infoln("Timeout set:", checkPodTimeout.String(), "for all daemonset pods to come online")

	for {
		select {
		case <- ctx.Done():
			errorMessage := "DaemonsetChecker: Node(s) which were unable to schedule before context was cancelled: " + formatNodes(nodesMissingDSPod)
			log.Errorln(errorMessage)
			return errors.New(errorMessage)
		default:
		}

		time.Sleep(time.Second)

		// if we need to shut down, stop waiting entirely
		if shuttingDown {
			errorMessage := "DaemonsetChecker: Node(s) which were unable to schedule before shutdown signal was received:" + formatNodes(nodesMissingDSPod)
			log.Errorln(errorMessage)
			return errors.New(errorMessage)
		}

		// check the number of nodes in the cluster.  Make sure we have that many
		// pods scheduled.

		// find nodes missing pods from this daemonset
		var err error
		nodesMissingDSPod, err = getNodesMissingDSPod()
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
				log.Infoln("DaemonsetChecker: Daemonset " + daemonSetName + " done deploying pods.")
				return nil
			}
			continue
		}
		// else if we've started counting up but there is a DS pod that went unready
		// reset the counter
		if counter > 0 {
			log.Infoln("DaemonsetChecker: Daemonset "+daemonSetName+" was ready for", counter, "out of,", readySeconds, "seconds but has left the ready state. Restarting", readySeconds, "second timer.")
			counter = 0
		}
		// If the counter isnt iterating up or being reset, we are still waiting for pods to come online
		log.Infoln("DaemonsetChecker: Daemonset check waiting for", len(nodesMissingDSPod), "pods to come up on nodes", nodesMissingDSPod)
	}
}

func formatNodes(nodeList []string) string {
	if len(nodeList) > 0 {
		return strings.Join(nodeList,", ")
	} else {
		return ""
	}
}

// getNodesMissingDSPod gets a list of nodes that do not have a DS pod running on them
func getNodesMissingDSPod() ([]string, error) {

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
		LabelSelector: "app=" + daemonSetName + ",source=kuberhealthy,khcheck=daemonset",
	})
	if err != nil {
		return nodesMissingDSPods, err
	}

	// populate a node status map. default status is "false", meaning there is
	// not a pod deployed to that node.  We are only adding nodes that tolerate
	// our list of dsc.Tolerations
	nodeStatuses := make(map[string]bool)
	for _, n := range nodes.Items {
		if taintsAreTolerated(n.Spec.Taints, tolerations) {
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
				if taintsAreTolerated(node.Spec.Taints, tolerations) {
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
func waitForAllDaemonsetsToClear() error {

	log.Infoln("Timeout set:", checkPodTimeout.String(), "for all daemonsets to clear")

	// watch events and return when the pod is in state running
	for {

		// wait between requests
		time.Sleep(time.Second * 5)

		// if the context is canceled, we stop
		select {
		case <- ctx.Done():
			return errors.New("Waiting for daemonset to clear was aborted by context cancellation")
		default:
		}

		// fetch the pod by name
		var err error
		daemonSetList, err = getAllDaemonsets()
		if err != nil {
			return err
		}
		if len(daemonSetList) == 0 {
			log.Info("All daemonsets cleared")
			return nil
		}
	}
}

// deleteDS deletes specified daemonset from its checkNamespace.
// Delete daemonset first, then proceed to delete all daemonset pods.
func deleteDS(dsName string) error {

	log.Infoln("DaemonsetChecker deleting daemonset:", dsName)

	// Confirm the count of ds pods we are removing before issuing a delete
	pods, err := listDSPods(dsName)
	if err != nil {
		return err
	}
	log.Infoln("There are", len(pods.Items), "daemonset pods to remove")

	// Delete daemonset
	dsClient := getDSClient()
	err = dsClient.Delete(dsName, &metav1.DeleteOptions{})
	if err != nil {
		errorMessage := "Failed to delete daemonset: " + dsName + err.Error()
		log.Errorln(errorMessage)
		return errors.New(errorMessage)
	}

	// Issue a delete to every pod. removing the DS alone does not ensure all pods are removed
	log.Infoln("DaemonsetChecker removing daemonset. Proceeding to remove daemonset pods")
	err = deleteDSPods(dsName)
	if err != nil {
		return err
	}

	return nil
}

// waitForPodRemoval waits for the daemonset to finish removing all daemonset pods
func waitForPodRemoval() error {
	log.Infoln("Timeout set:", checkPodTimeout.String(), "for daemonset removal")

	// as a fix for kuberhealthy #74 we routinely ask the pods to remove.
	// this is a workaround for a race in kubernetes that sometimes leaves
	// daemonset pods in a 'Ready' state after the daemonset has been deleted
	deleteTicker := time.NewTicker(time.Second * 30)

	// loop until all daemonset pods are deleted
	for {
		// check for our context to expire to break the loop
		ctxErr := ctx.Err()
		if ctxErr != nil {
			return ctxErr
		}

		var err error
		podRemovalList, err = listDSPods(daemonSetName)
		if err != nil {
			return err
		}

		log.Infoln("DaemonsetChecker using LabelSelector: app="+daemonSetName+",source=kuberhealthy,khcheck=daemonset to remove ds pods")

		// If the delete ticker has ticked, then issue a repeat request for pods to be deleted.
		// See kuberhealthy issue #74
		select {
		case <-deleteTicker.C:
			log.Infoln("DaemonsetChecker re-issuing a pod delete command for daemonset checkers.")
			err = deleteDSPods(daemonSetName)
			if err != nil {
				return err
			}
		default:
		}

		// Check all pods for any kuberhealthy test daemonset pods that still exist
		log.Infoln("DaemonsetChecker waiting for", len(podRemovalList.Items), "pods to delete")
		for _, p := range podRemovalList.Items {
			log.Infoln("DaemonsetChecker is still removing:", p.Namespace, p.Name, "on node", p.Spec.NodeName)
		}

		if len(podRemovalList.Items) == 0 {
			log.Infoln("DaemonsetChecker has finished removing all daemonset pods")
			return nil
		}
		time.Sleep(time.Second * 1)
	}
}


// waitForDSRemoval waits for the daemonset to be removed before returning
func waitForDSRemoval() error {
	log.Infoln("Timeout set:", checkPodTimeout.String(), "for daemonset pods removal")

	// repeatedly fetch the DS until it goes away
	for {
		select {
		case <- ctx.Done():
			return errors.New("Waiting for daemonset: " + daemonSetName + " removal aborted by context cancellation.")
		default:
		}
		// check for our context to expire to break the loop
		ctxErr := ctx.Err()
		if ctxErr != nil {
			return ctxErr
		}
		time.Sleep(time.Second / 2)
		exists, err := fetchDS(daemonSetName)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
	}
}

// fetchDS fetches the ds for the checker from the api server
// and returns a bool indicating if it exists or not
func fetchDS(dsName string) (bool, error) {
	dsClient := getDSClient()
	var firstQuery bool
	var more string
	// pagination
	for firstQuery || len(more) > 0 {
		firstQuery = false
		dsList, err := dsClient.List(metav1.ListOptions{
			Continue: more,
		})
		if err != nil {
			errorMessage := "Failed to fetch daemonset: " + err.Error()
			log.Errorln(errorMessage)
			return false, errors.New(errorMessage)
		}
		more = dsList.Continue

		// check results for our daemonset
		for _, item := range dsList.Items {
			if item.GetName() == dsName {
				// ds does exist, return true
				return true, nil
			}
		}
	}

	// daemonset does not exist, return false
	return false, nil
}

// listDSPods lists all daemonset pods
func listDSPods(dsName string) (*apiv1.PodList, error) {
	podClient := getPodClient()
	var podList *apiv1.PodList
	var err error
	podList, err = podClient.List(metav1.ListOptions{
		LabelSelector: "app=" + dsName + ",source=kuberhealthy,khcheck=daemonset",
	})
	if err != nil {
		errorMessage := "Failed to list daemonset: " + dsName + " pods: " + err.Error()
		log.Errorln(errorMessage)
		return podList, errors.New(errorMessage)
	}
	return podList, err
}

// deleteDSPods issues a delete on all daemonset pods
func deleteDSPods(dsName string) error {
	podClient := getPodClient()
	err := podClient.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: "app=" + dsName + ",source=kuberhealthy,khcheck=daemonset",
	})
	if err != nil {
		errorMessage := "Failed to delete daemonset " + dsName + " pods: " + err.Error()
		log.Errorln(errorMessage)
		return errors.New(errorMessage)
	}
	return err
}

// deletePod deletes a pod with the specified name
func deletePod(podName string) error {
	podClient := getPodClient()
	propagationForeground := metav1.DeletePropagationForeground
	options := &metav1.DeleteOptions{PropagationPolicy: &propagationForeground}
	err := podClient.Delete(podName, options)
	return err
}

// getUnClearedDSList transforms list of daemonsets to a list of daemonset name strings. Used for error messaging.
func getUnClearedDSList(dsList []appsv1.DaemonSet) []string {

	var unclearedDS []string
	if len(dsList) != 0 {
		for _, ds := range dsList {
			unclearedDS = append(unclearedDS, ds.Name)
		}
	}

	return unclearedDS
}

// getDSPodsNodeList transforms podList to a list of pod node name strings. Used for error messaging.
func getDSPodsNodeList(podList *apiv1.PodList) []string {

	var nodeList []string
	if len(podList.Items) != 0 {
		for _, p := range podList.Items {
			nodeList = append(nodeList, p.Spec.NodeName)
		}
	}

	return nodeList
}