package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/util"
)

// Globals for revealing daemonsets that fail to be removed or
// nodes with daemonset pods that fail to come up or be removed
var nodesMissingDSPod []string
var podRemovalList *apiv1.PodList

// runCheck runs pre-check cleanup and then the full daemonset check
func runCheck(ctx context.Context) error {

	log.Infoln("Running daemonset check")
	err := runDaemonsetCheck(ctx)
	if err != nil {
		return err
	}
	return nil
}

// runDaemonsetCheck runs the full daemonset check. Deploy daemonset, remove daemonset, and post-check cleanup.
func runDaemonsetCheck(ctx context.Context) error {

	// Deploy Daemonset
	log.Infoln("Running daemonset deploy...")
	err := deploy(ctx)
	if err != nil {
		return err
	}

	// remove the daemonset and block until completed
	log.Infoln("Running daemonset removal...")
	err = remove(ctx, daemonSetName)
	if err != nil {
		return err
	}

	return nil
}

// deploy runs doDeploy and checks for any errors during the deployment.
func deploy(ctx context.Context) error {
	log.Infoln("Deploying daemonset.")

	// do the deployment and try to clean up if it fails
	err := doDeploy(ctx)
	if err != nil {
		return fmt.Errorf("error deploying daemonset: %s", err)
	}

	// wait for pods to come online
	doneChan := make(chan error, 1)
	go func() {
		log.Debugln("Worker: waitForPodsToComeOnline started")
		doneChan <- waitForPodsToComeOnline(ctx)
	}()

	// set daemonset deploy deadline
	deadlineChan := time.After(checkDeadline.Sub(now))
	// watch for either the pods to come online, the check timeout to pass, or the context to be revoked
	select {
	case err = <-doneChan:
		if err != nil {
			return fmt.Errorf("error waiting for pods to come online: %s", err)
		}
		log.Infoln("Successfully deployed daemonset.")
	case <-deadlineChan:
		log.Debugln("nodes missing DS pods:", nodesMissingDSPod)
		return errors.New("Reached check pod timeout: " + checkDeadline.Sub(now).String() + " waiting for all pods to come online. " +
			"Node(s) missing daemonset pod: " + formatNodes(nodesMissingDSPod))
	case <-ctx.Done():
		return errors.New("failed to complete check due to an interrupt signal. canceling deploying daemonset and shutting down from interrupt")
	}
	return nil
}

// doDeploy creates a daemonset
func doDeploy(ctx context.Context) error {
	//Generate the spec for the DS that we are about to deploy
	daemonSetSpec := generateDaemonSetSpec(ctx)

	//Generate DS client and create the set with the template we just generated
	err := createDaemonset(ctx, daemonSetSpec)
	return err
}

// remove removes the created daemonset for this check from the cluster. Waits for daemonset and daemonset pods to clear
func remove(ctx context.Context, dsName string) error {
	log.Infoln("Removing daemonset.")

	doneChan := make(chan error, 1)
	// start the DS delete in the background
	go func() {
		doneChan <- deleteDS(ctx, dsName)
	}()

	// set daemonset remove deadline
	deadlineChan := time.After(checkDeadline.Sub(now))
	// wait for the DS delete call to finish, the timeout to happen, or the context to cancel
	select {
	case err := <-doneChan:
		if err != nil {
			return fmt.Errorf("error trying to delete daemonset: %s", err)
		}
		log.Infoln("Successfully requested daemonset removal.")
	case <-deadlineChan:
		return errors.New("Reached check pod timeout: " + checkDeadline.Sub(now).String() + " waiting for daemonset removal command to complete.")
	case <-ctx.Done():
		// If there is a cancellation interrupt signal.
		return errors.New("failed to complete check due to shutdown signal. canceling daemonset removal and shutting down from interrupt")
	}

	// Wait for daemonset to be removed
	go func() {
		log.Debugln("Worker: waitForDSRemoval started")
		doneChan <- waitForDSRemoval(ctx)
	}()

	// wait for either the DS to be removed, the timeout to occur, or a context cancellation
	select {
	case err := <-doneChan:
		if err != nil {
			return fmt.Errorf("error waiting for daemonset removal: %s", err)
		}
		log.Infoln("Successfully removed daemonset.")
	case <-deadlineChan:
		return errors.New("Reached check pod timeout: " + checkDeadline.Sub(now).String() + " waiting for daemonset removal.")
	case <-ctx.Done():
		// If there is a cancellation interrupt signal.
		return errors.New("failed to complete check due to an interrupt signal. canceling removing daemonset and shutting down from interrupt")
	}

	// Wait for all daemonsets pods to be removed
	go func() {
		log.Debugln("Worker: waitForPodRemoval started")
		doneChan <- waitForPodRemoval(ctx)
	}()

	// wait for all pods to be removed, a timeout, or the context to revoke
	select {
	case err := <-doneChan:
		if err != nil {
			return fmt.Errorf("error waiting for daemonset pods removal: %s", err)
		}
		log.Infoln("Successfully removed daemonset pods.")
	case <-deadlineChan:
		unClearedDSPodsNodes := getDSPodsNodeList(podRemovalList)
		return errors.New("reached check pod timeout: " + checkDeadline.Sub(now).String() + " waiting for daemonset pods removal. " + "Node(s) failing to remove daemonset pod: " + unClearedDSPodsNodes)
	case <-ctx.Done():
		return errors.New("failed to complete check due to an interrupt signal. canceling removing daemonset pods and shutting down from interrupt")
	}

	return nil
}

// waitForPodsToComeOnline blocks until all pods of the daemonset are deployed and online
func waitForPodsToComeOnline(ctx context.Context) error {

	log.Debugln("Waiting for all ds pods to come online")

	// counter for DS status check below
	var counter int

	// init a timeout for this whole deletion of daemonsets
	log.Infoln("Timeout set:", checkDeadline.Sub(now).String(), "for all daemonset pods to come online")

	for {
		select {
		case <-ctx.Done():
			return errors.New("DaemonsetChecker: Node(s) which were unable to schedule before context was cancelled: " + formatNodes(nodesMissingDSPod))
		default:
		}

		time.Sleep(time.Second)

		// check the number of nodes in the cluster.  Make sure we have that many
		// pods scheduled.

		// find nodes missing pods from this daemonset
		var err error
		nodesMissingDSPod, err = getNodesMissingDSPod(ctx)
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
		log.Infoln("DaemonsetChecker: Daemonset check waiting for", len(nodesMissingDSPod), "pod(s) to come up on nodes", nodesMissingDSPod)
	}
}

// waitForDSRemoval waits for the daemonset to be removed before returning
func waitForDSRemoval(ctx context.Context) error {

	log.Debugln("Waiting for ds removal")

	// repeatedly fetch the DS until it goes away
	for {
		select {
		case <-ctx.Done():
			return errors.New("Waiting for daemonset: " + daemonSetName + " removal aborted by context cancellation.")
		default:
		}
		// check for our context to expire to break the loop
		ctxErr := ctx.Err()
		if ctxErr != nil {
			return ctxErr
		}
		time.Sleep(time.Second / 2)
		exists, err := fetchDS(ctx, daemonSetName)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
	}
}

// waitForPodRemoval waits for the daemonset to finish removing all daemonset pods
func waitForPodRemoval(ctx context.Context) error {

	log.Debugln("Waiting for ds pods removal")

	// as a fix for kuberhealthy #74 we routinely ask the pods to remove.
	// this is a workaround for a race in kubernetes that sometimes leaves
	// daemonset pods in a 'Ready' state after the daemonset has been deleted
	deleteTicker := time.NewTicker(time.Second * 30)

	// loop until all daemonset pods are deleted
	for {

		var err error
		podRemovalList, err = listPods(ctx)
		if err != nil {
			errorMessage := "Failed to list daemonset: " + daemonSetName + " pods: " + err.Error()
			log.Errorln(errorMessage)
			return errors.New(errorMessage)
		}

		log.Infoln("DaemonsetChecker using LabelSelector: kh-app=" + daemonSetName + ",source=kuberhealthy,khcheck=daemonset to remove ds pods")

		// If the delete ticker has ticked, then issue a repeat request for pods to be deleted.
		// See kuberhealthy issue #74
		select {
		case <-deleteTicker.C:
			log.Infoln("DaemonsetChecker re-issuing a pod delete command for daemonset checkers.")
			err := deletePods(ctx, daemonSetName)
			if err != nil {
				errorMessage := "Failed to delete daemonset " + daemonSetName + " pods: " + err.Error()
				log.Errorln(errorMessage)
				return errors.New(errorMessage)
			}
		case <-ctx.Done():
			return errors.New("timed out when waiting for pod removal")
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

// generateDaemonSetSpec generates a daemonset spec to deploy into the cluster
func generateDaemonSetSpec(ctx context.Context) *appsv1.DaemonSet {

	checkRunTime := strconv.Itoa(int(now.Unix()))
	terminationGracePeriod := int64(1)

	// Set the runAsUser
	runAsUser := defaultUser
	currentUser, err := util.GetCurrentUser(defaultUser)
	if err != nil {
		log.Errorln("Unable to get the current user id ", err)
	}
	log.Debugln("runAsUser will be set to ", currentUser)
	runAsUser = currentUser

	// if a list of tolerations wasnt passed in, default to tolerating all taints
	if len(tolerations) == 0 {
		// find all the taints in the cluster and create a toleration for each
		tolerations, err = findAllUniqueTolerations(ctx, client)
		if err != nil {
			log.Warningln("Unable to generate list of pod scheduling tolerations", err)
		}
	}

	// Add daemonset check pod ownerReference
	ownerRef, err := util.GetOwnerRef(client, checkNamespace)
	if err != nil {
		log.Errorln("Error getting ownerReference:", err)
	}

	// Check for given node selector values.
	// Set the map to the default of nil (<none>) if there are no selectors given.
	if len(dsNodeSelectors) == 0 {
		dsNodeSelectors = nil
	}

	// create the DS object
	log.Infoln("Generating daemonset kubernetes spec.")
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: daemonSetName,
			Labels: map[string]string{
				"kh-app":           daemonSetName,
				"source":           "kuberhealthy",
				"khcheck":          "daemonset",
				"creatingInstance": hostName,
				"checkRunTime":     checkRunTime,
			},
			OwnerReferences: ownerRef,
		},
		Spec: appsv1.DaemonSetSpec{
			MinReadySeconds: 2,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"kh-app":           daemonSetName,
					"source":           "kuberhealthy",
					"khcheck":          "daemonset",
					"creatingInstance": hostName,
					"checkRunTime":     checkRunTime,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"kh-app":           daemonSetName,
						"source":           "kuberhealthy",
						"khcheck":          "daemonset",
						"creatingInstance": hostName,
						"checkRunTime":     checkRunTime,
					},
					Name: daemonSetName,
					Annotations: map[string]string{
						"cluster-autoscaler.kubernetes.io/safe-to-evict": "true",
					},
					OwnerReferences: ownerRef,
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Tolerations:                   []apiv1.Toleration{},
					PriorityClassName:             podPriorityClassName,
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
					NodeSelector: dsNodeSelectors,
				},
			},
		},
	}

	// Add our generated list of tolerations or any the user input via flag
	daemonSet.Spec.Template.Spec.Tolerations = append(daemonSet.Spec.Template.Spec.Tolerations, tolerations...)
	log.Infoln("Deploying daemonset with tolerations: ", daemonSet.Spec.Template.Spec.Tolerations)

	return daemonSet
}

// findAllUniqueTolerations returns a list of all taints present on any node group in the cluster
// this is exportable because of a chicken/egg.  We need to determine the taints before
// we construct the testDS in New() and pass them into New()
func findAllUniqueTolerations(ctx context.Context, client *kubernetes.Clientset) ([]apiv1.Toleration, error) {

	var uniqueTolerations []apiv1.Toleration

	// get a list of all the nodes in the cluster
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return uniqueTolerations, err
	}
	log.Debugln("Searching for unique taints on the cluster.")
	// this keeps track of the unique taint values
	keys := make(map[string]bool)
	// get a list of all taints
	for _, n := range nodes.Items {
		for _, t := range n.Spec.Taints {

			// Don't tolerate any taints listed in ALLOWED_TAINTS
			// Ignoring cordoned nodes example: node.kubernetes.io/unschedulable:NoSchedule
			if val, exists := allowedTaints[t.Key]; exists {
				if val == t.Effect {
					// Skip tolerating allowed taints
					continue
				}
			}

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

// getNodesMissingDSPod gets a list of nodes that do not have a DS pod running on them
func getNodesMissingDSPod(ctx context.Context) ([]string, error) {

	// nodesMissingDSPods holds the final list of nodes missing pods
	var nodesMissingDSPods []string

	// get a list of all the nodes in the cluster
	nodes, err := listNodes(ctx)
	if err != nil {
		errorMessage := "Failed to list nodes: " + err.Error()
		log.Errorln(errorMessage)
		return nodesMissingDSPods, errors.New(errorMessage)
	}

	// get a list of DS pods
	pods, err := listPods(ctx)
	if err != nil {
		errorMessage := "Failed to list daemonset: " + daemonSetName + " pods: " + err.Error()
		log.Errorln(errorMessage)
		return nodesMissingDSPods, errors.New(errorMessage)
	}

	// populate a node status map. default status is "false", meaning there is
	// not a pod deployed to that node.  We are only adding nodes that tolerate
	// our list of dsc.Tolerations
	nodeStatuses := make(map[string]bool)
	for _, n := range nodes.Items {
		if taintsAreTolerated(n.Spec.Taints, tolerations) && nodeLabelsMatch(n.Labels, dsNodeSelectors) {
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

// nodeLabelsMatch iterates through labels on a node and checks for matches
// with given node selectors
func nodeLabelsMatch(labels, nodeSelectors map[string]string) bool {
	labelsMatch := true // assume that the node selectors match node labels
	// look at given node selector keys and values
	for selectorKey, selectorValue := range nodeSelectors {
		// check if the node has a similar label key
		labelValue, ok := labels[selectorKey]
		// if there is no matching key, continue to the next node selector
		if !ok {
			labelsMatch = false
			continue
		}
		// if there is a matching key, the label's value should match the node selector's value
		// otherwise, this node does not match
		if labelValue != selectorValue {
			labelsMatch = false
		}
	}
	return labelsMatch
}

// deleteDS deletes specified daemonset from its checkNamespace.
// Delete daemonset first, then proceed to delete all daemonset pods.
func deleteDS(ctx context.Context, dsName string) error {

	log.Infoln("DaemonsetChecker deleting daemonset:", dsName)

	// Confirm the count of ds pods we are removing before issuing a delete
	pods, err := listPods(ctx)
	if err != nil {
		errorMessage := "Failed to list daemonset: " + daemonSetName + " pods: " + err.Error()
		log.Errorln(errorMessage)
		return errors.New(errorMessage)
	}
	log.Infoln("There are", len(pods.Items), "daemonset pods to remove")

	// Delete daemonset
	err = deleteDaemonset(ctx, dsName)
	if err != nil {
		errorMessage := "Failed to delete daemonset: " + dsName + err.Error()
		log.Errorln(errorMessage)
		return errors.New(errorMessage)
	}

	// Issue a delete to every pod. removing the DS alone does not ensure all pods are removed
	log.Infoln("DaemonsetChecker removing daemonset. Proceeding to remove daemonset pods")
	err = deletePods(ctx, dsName)
	if err != nil {
		errorMessage := "Failed to delete daemonset " + dsName + " pods: " + err.Error()
		log.Errorln(errorMessage)
		return errors.New(errorMessage)
	}

	return nil
}

// fetchDS fetches the ds for the checker from the api server
// and returns a bool indicating if it exists or not

func fetchDS(ctx context.Context, dsName string) (bool, error) {
	var firstQuery bool = true
	var more string
	// pagination
	for firstQuery || len(more) > 0 {
		firstQuery = false
		dsList, err := listDaemonsets(ctx, more)
		if err != nil {
			log.Errorln(err.Error())
			return false, err
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
