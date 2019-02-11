// Package daemonSet contains a Kuberhealthy check for the ability to roll out
// a daemonset to a cluster.  Includes validation of cleanup as well.  This
// check provides a high level of confidence that the cluster is operating
// normally.
package daemonSet // import "github.com/Comcast/kuberhealthy/pkg/checks/daemonSet"

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	apiv1 "k8s.io/api/core/v1"
	betaapiv1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
)

// TODO - ingest daemonset name and namespace with flags?
const daemonSetBaseName = "daemonset-test"

var namespace = os.Getenv("POD_NAMESPACE")

// Checker implements a KuberhealthyCheck for daemonset
// deployment and teardown checking.
type Checker struct {
	Namespace         string
	DaemonSet         *betaapiv1.DaemonSet
	ErrorMessages     []string
	shuttingDown      bool
	DaemonSetDeployed bool
	DaemonSetName     string
	hostname          string
	tolerations       []apiv1.Toleration
	client            *kubernetes.Clientset
}

// New creates a new Checker object
func New() (*Checker, error) {

	hostname := getHostname()
	var tolerations []apiv1.Toleration

	testDS := Checker{
		ErrorMessages: []string{},
		Namespace:     namespace,
		DaemonSetName: daemonSetBaseName + "-" + hostname + "-" + strconv.Itoa(int(time.Now().Unix())),
		hostname:      hostname,
		tolerations:   tolerations,
	}

	return &testDS, nil
}

// generateDaemonSetSpec generates a daemon set spec to deploy into the cluster
func (dsc *Checker) generateDaemonSetSpec() {

	terminationGracePeriod := int64(1)

	// find all the taints in the cluster and create a toleration for each
	var err error
	dsc.tolerations, err = findAllUniqueTolerations(dsc.client)
	if err != nil {
		log.Warningln("Unable to generate list of pod scheduling tolerations", err)
	}

	//create the DS object
	log.Infoln("Generating daemon set kubernetes spec.")
	dsc.DaemonSet = &betaapiv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: dsc.DaemonSetName,
			Labels: map[string]string{
				"app":              dsc.DaemonSetName,
				"source":           "kuberhealthy",
				"creatingInstance": dsc.hostname,
			},
		},
		Spec: betaapiv1.DaemonSetSpec{
			MinReadySeconds: 2,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":              dsc.DaemonSetName,
					"source":           "kuberhealthy",
					"creatingInstance": dsc.hostname,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":              dsc.DaemonSetName,
						"source":           "kuberhealthy",
						"creatingInstance": dsc.hostname,
					},
					Name: dsc.DaemonSetName,
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Tolerations:                   []apiv1.Toleration{},
					Containers: []apiv1.Container{
						apiv1.Container{
							Name:  "sleep",
							Image: "gcr.io/google_containers/pause:0.8.0",
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
	dsc.DaemonSet.Spec.Template.Spec.Tolerations = append(dsc.DaemonSet.Spec.Template.Spec.Tolerations, dsc.tolerations...)
	log.Infoln("Deploying daemon set with tolerations: ", dsc.DaemonSet.Spec.Template.Spec.Tolerations)
}

// Name returns the name of this checker
func (dsc *Checker) Name() string {
	return "DaemonSetChecker"
}

// CheckNamespace returns the namespace of this checker
func (dsc *Checker) CheckNamespace() string {
	return dsc.Namespace
}

// Interval returns the interval at which this check runs
func (dsc *Checker) Interval() time.Duration {
	return time.Minute * 15
}

// Timeout returns the maximum run time for this check before it times out
func (dsc *Checker) Timeout() time.Duration {
	return time.Minute * 10
}

// Shutdown signals the DS to begin a cleanup
func (dsc *Checker) Shutdown() error {
	dsc.shuttingDown = true

	// make a context to satisfy pod removal
	ctx := context.Background()
	ctx, cancelCtx := context.WithCancel(ctx)

	// cancel the shutdown context after the timeout
	go func() {
		<-time.After(dsc.Timeout())
		cancelCtx()
	}()

	// if the ds is deployed, delete it
	if dsc.DaemonSetDeployed {
		dsc.remove()
		dsc.waitForPodRemoval(ctx)
	}

	log.Infoln(dsc.Name(), "Daemonset "+dsc.DaemonSetName+" ready for shutdown.")
	return nil

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

// CurrentStatus returns the status of the check as of right now
func (dsc *Checker) CurrentStatus() (bool, []string) {
	if len(dsc.ErrorMessages) > 0 {
		return false, dsc.ErrorMessages
	}
	return true, dsc.ErrorMessages
}

// clearErrors clears all errors from the checker
func (dsc *Checker) clearErrors() {
	dsc.ErrorMessages = []string{}
}

// cleanupOrphans cleans up orphaned pods and daemonsets, if they exist
func (dsc *Checker) cleanupOrphans() error {

	// first, clean up daemonsets
	err := dsc.cleanupOrphanedDaemonsets()
	if err != nil {
		return err
	}

	// we must also remove pods directly because they sometimes are able to exist
	// even when their creating daemonset has been removed.
	err = dsc.cleanupOrphanedPods()
	return err
}

// cleanupOrphanedPods cleans up daemonset pods that shouldn't exist because their
// creating instance is gone.  Sometimes removing daemonsets isnt enough to clean up
// orphaned pods.
func (dsc *Checker) cleanupOrphanedPods() error {
	pods, err := dsc.getAllPods()
	if err != nil {
		log.Errorln("Error fetching pods:", err)
		return err
	}

	// loop on all the daemonsets and ensure that daemonset's creating pod exists.
	// if the creating pod does not exist, then we delete the daemonset.
	for _, p := range pods {
		log.Infoln("Checking if pod is orphaned:", p.Name, "creatingInstance:", p.Labels["creatingInstance"])

		// fetch the creatingInstance label
		creatingDSInstance := p.Labels["app"]

		// if there isnt a creatingInstance label, we assume its an old generation and remove it.
		if len(creatingDSInstance) == 0 {
			log.Warningln("Unable to find app label on pod", p.Name, "assuming orphaned and removing!")
			err := dsc.deletePod(p.Name)
			if err != nil {
				log.Warningln("error when removing orphaned pod due to missing label", p.Name+": ", err)
			}
			continue
		}

		// check if the creatingInstance exists
		exists := dsc.checkIfDSExists(creatingDSInstance)
		if err != nil {
			log.Errorln("error checking if kuberhealthy daemonset exists:", err)
			return err
		}

		// if the owning kuberhealthy pod of the DS does not exist, then we delete the daemonset
		if !exists {
			log.Infoln("Removing orphaned pod", p.Name, "because kuberhealthy ds", creatingDSInstance, "does not exist")
			err := dsc.deletePod(p.Name)
			if err != nil {
				log.Warningln("error when removing orphaned pod", p.Name+": ", err)
				return err
			}
		}
	}
	return nil
}

// cleanupOrphanedDaemonsets cleans up daemonsets that should not exist based on their
// creatingInstance label.
func (dsc *Checker) cleanupOrphanedDaemonsets() error {

	daemonSets, err := dsc.getAllDaemonsets()
	if err != nil {
		log.Errorln("Error fetching daemonsets for cleanup:", err)
		return err
	}

	// loop on all the daemonsets and ensure that daemonset's creating pod exists.
	// if the creating pod does not exist, then we delete the daemonset.
	for _, ds := range daemonSets {
		log.Infoln("Checking if daemonset is orphaned:", ds.Name, "creatingInstance:", ds.Labels["creatingInstance"])

		// fetch the creatingInstance label
		creatingInstance := ds.Labels["creatingInstance"]

		// if there isnt a creatingInstance label, we assume its an old generation and remove it.
		if len(creatingInstance) == 0 {
			log.Warningln("Unable to find hostname with creatingInstance label on ds", ds.Name, "assuming orphaned and removing!")
			err := dsc.deleteDS(ds.Name)
			if err != nil {
				log.Warningln("error when removing orphaned daemonset due to missing label", ds.Name+": ", err)
				return err
			}
			continue
		}

		// check if the creatingInstance exists
		exists := dsc.checkIfPodExists(creatingInstance)
		if err != nil {
			log.Errorln("error checking if kuberhealthy ds exists:", err)
			return err
		}

		// if the owning kuberhealthy pod of the DS does not exist, then we delete the daemonset
		if !exists {
			log.Infoln("Removing orphaned daemonset", ds.Name, "because creating kuberhealthy instance", creatingInstance, "does not exist")
			err := dsc.deleteDS(ds.Name)
			if err != nil {
				log.Warningln("error when removing orphaned daemonset", ds.Name+": ", err)
				return err
			}
		}
	}
	return nil
}

// deleteDS deletes the DS with the specified name
func (dsc *Checker) deleteDS(dsName string) error {

	propagationForeground := metav1.DeletePropagationForeground
	dsClient := dsc.getDaemonSetClient()
	err := dsClient.Delete(dsName, &metav1.DeleteOptions{PropagationPolicy: &propagationForeground})
	return err
}

// deletePod deletes a pod with the specified name
func (dsc *Checker) deletePod(podName string) error {
	propagationForeground := metav1.DeletePropagationForeground
	options := &metav1.DeleteOptions{PropagationPolicy: &propagationForeground}
	err := dsc.client.CoreV1().Pods(dsc.Namespace).Delete(podName, options)
	return err
}

// checkIfDSExists fetches a specific kuberhealthy ds by name to ensure
// it exists.
func (dsc *Checker) checkIfDSExists(dsName string) bool {
	dsClient := dsc.getDaemonSetClient()
	_, err := dsClient.Get(dsName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}

// checkIfPodExists fetches a specific kuberhealthy pod by name to ensure
// it exists.
func (dsc *Checker) checkIfPodExists(podName string) bool {
	_, err := dsc.client.CoreV1().Pods(dsc.Namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}

// getAllPods fetches all pods in the namespace, for all instances of kuberhealthy
// based on a source=kuberhealthy label.
func (dsc *Checker) getAllPods() ([]apiv1.Pod, error) {

	var allPods []apiv1.Pod
	var cont string

	// fetch the pod objects created by kuberhealthy
	for {
		var podList *apiv1.PodList
		podList, err := dsc.client.Core().Pods(dsc.Namespace).List(metav1.ListOptions{
			LabelSelector: "source=kuberhealthy",
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

// getAllDaemonsets fetches all daemonsets in the namespace, for all
// instances of kuberhealthy
func (dsc *Checker) getAllDaemonsets() ([]betaapiv1.DaemonSet, error) {

	var allDS []betaapiv1.DaemonSet
	var cont string
	var err error

	// fetch the ds objects created by kuberhealthy
	for {
		var dsList *betaapiv1.DaemonSetList
		dsClient := dsc.getDaemonSetClient()
		dsList, err = dsClient.List(metav1.ListOptions{
			LabelSelector: "source=kuberhealthy",
		})
		if err != nil {
			log.Warningln("Unable to get all Daemon Sets:", err)
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

// Run implements the entrypoint for check execution
func (dsc *Checker) Run(client *kubernetes.Clientset) error {

	// make a context for this run
	ctx, cancelCtx := context.WithCancel(context.Background())

	doneChan := make(chan error)

	dsc.client = client

	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := dsc.doChecks(ctx)
		doneChan <- err
	}(doneChan)

	// wait for either a timeout or job completion
	select {
	case <-time.After(dsc.Interval()):
		// The check has timed out because its time to run again
		cancelCtx() // cancel context
		errorMessage := "Failed to complete checks for " + dsc.Name() + " in time!  Next run came up but check was still running."
		dsc.ErrorMessages = []string{errorMessage}
		log.Errorln(dsc.Name(), errorMessage)
	case <-time.After(dsc.Timeout()):
		// The check has timed out after its specified timeout period
		cancelCtx() // cancel context
		errorMessage := "Failed to complete checks for " + dsc.Name() + " in time!  Timeout was reached."
		dsc.ErrorMessages = []string{errorMessage}
		log.Errorln(dsc.Name(), errorMessage)
	case err := <-doneChan:
		cancelCtx()
		return err
	}

	return nil
}

// doChecks actually runs checking procedures
func (dsc *Checker) doChecks(ctx context.Context) error {

	// clean up any existing daemonsets that may be laying around
	// waiting so not to cause a conflict.  Don't listen to errors here.
	dsc.cleanUp(ctx)

	// deploy the daemonset
	err := dsc.doDeploy(ctx)
	if err != nil {
		return err
	}

	// clean up the daemonset.  Does not return until removed completely or
	// an error occurs
	err = dsc.doRemove(ctx)
	if err != nil {
		return err
	}

	dsc.clearErrors() // clear errors if checks are all good

	// fire off an orphan cleanup in the background on each check run
	go dsc.cleanupOrphans()

	return nil
}

// cleanUp finds and removes any existing daemonsets in case they are
// left abandoned from a race condition.
func (dsc *Checker) cleanUp(ctx context.Context) error {

	// get a DS client
	dsClient := dsc.getDaemonSetClient()

	// check for existing daemonset to cleanup
	ds, err := dsClient.Get(dsc.DaemonSetName, metav1.GetOptions{})

	// if a DS isn't found, then return nil. No cleanup is needed.
	if err != nil && strings.Contains(err.Error(), "not found") {
		return nil
	}
	if err != nil {
		return err
	}

	// if a DS exists, then clean it up
	if ds.Name != "" {
		log.Warningln("Rogue or leftover daemonset.  Removing before running checks")

		// if there wasnt an error, the DS exists and we need to clean it up.
		err = dsc.remove()
		if err != nil {
			return err
		}

		// watch for the daemonset to not exist before returning
		err = dsc.waitForDSRemoval(ctx)
		if err != nil {
			return err
		}

		// wait for ds pods to be deleted
		err = dsc.waitForPodRemoval(ctx)
		return err
	}

	return nil

}

// waitForDSRemoval waits for the daemonset to be removed before returning
func (dsc *Checker) waitForDSRemoval(ctx context.Context) error {
	// repeatedly fetch the DS until it goes away
	for {
		// check for our context to expire to break the loop
		ctxErr := ctx.Err()
		if ctxErr != nil {
			return ctxErr
		}
		time.Sleep(time.Second / 2)
		exists, err := dsc.fetchDS()
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
func (dsc *Checker) fetchDS() (bool, error) {
	dsClient := dsc.getDaemonSetClient()
	var firstQuery bool
	var more string
	// pagination
	for firstQuery || len(more) > 0 {
		firstQuery = false
		dsList, err := dsClient.List(metav1.ListOptions{
			Continue: more,
		})
		if err != nil {
			return false, err
		}
		more = dsList.Continue

		// check results for our daemonset
		for _, item := range dsList.Items {
			if item.GetName() == dsc.DaemonSetName {
				// ds does exist, return true
				return true, nil
			}
		}
	}

	// daemonset does not exist, return false
	return false, nil
}

// doDeploy actually deploys the DS into the cluster
func (dsc *Checker) doDeploy(ctx context.Context) error {

	// create DS
	dsc.DaemonSetDeployed = true
	err := dsc.deploy()
	if err != nil {
		dsc.doRemove(ctx)
		return err
	}

	// wait for ds pods to be created
	err = dsc.waitForPodsToComeOnline(ctx)
	return err
}

// doRemove remotes the daemonset from the cluster
func (dsc *Checker) doRemove(ctx context.Context) error {
	// delete ds
	err := dsc.remove()
	if err != nil {
		return err
	}

	// wait for daemonset to be removed
	err = dsc.waitForDSRemoval(ctx)
	if err != nil {
		return err
	}

	// wait for ds pods to be deleted
	err = dsc.waitForPodRemoval(ctx)
	dsc.DaemonSetDeployed = true
	return err
}

// waitForPodsToComeOnline blocks until all pods of the daemonset are deployed and online
func (dsc *Checker) waitForPodsToComeOnline(ctx context.Context) error {

	// counter for DS status check below
	var counter int
	var nodesMissingDSPod []string

	for {
		ctxErr := ctx.Err()
		if ctxErr != nil {
			log.Infoln(dsc.Name(), "Nodes which were unable to schedule before context was cancelled:", nodesMissingDSPod)
			return ctxErr
		}
		time.Sleep(time.Second)

		// if we need to shut down, stop waiting entirely
		if dsc.shuttingDown {
			log.Infoln(dsc.Name(), "Nodes which were unable to schedule before shutdown signal was received:", nodesMissingDSPod)
			return nil
		}

		// check the number of nodes in the cluster.  Make sure we have that many
		// pods scheduled.

		// find nodes missing pods from this daemonset
		nodesMissingDSPod, err := dsc.getNodesMissingDSPod()
		if err != nil {
			log.Warningln(dsc.Name(), "Error determining which node was unschedulable. Retrying.", err)
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
			log.Infoln("All daemonset pods have been ready for", counter, "/", readySeconds, "seconds.")
			if counter >= readySeconds {
				log.Infoln(dsc.Name(), "Daemonset "+dsc.DaemonSetName+" done deploying pods.")
				return nil
			}
			continue
		}
		// else if we've started counting up but there is a DS pod that went unready
		// reset the counter
		if counter > 0 {
			log.Infoln(dsc.Name(), "Daemonset "+dsc.DaemonSetName+" was ready for", counter, "out of,", readySeconds, "seconds but has left the ready state. Restarting", readySeconds, "second timer.")
			counter = 0
		}
		// If the counter isnt iterating up or being reset, we are still waiting for pods to come online
		log.Infoln(dsc.Name(), "Daemonset check waiting for", len(nodesMissingDSPod), "pods to come up on nodes", nodesMissingDSPod)
	}
}

// getNodesMissingDSPod gets a list of nodes that do not have a DS pod running on them
func (dsc *Checker) getNodesMissingDSPod() ([]string, error) {

	// nodesMissingDSPods holds the final list of nodes missing pods
	var nodesMissingDSPods []string

	// get a list of all the nodes in the cluster
	nodes, err := dsc.client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nodesMissingDSPods, err
	}

	// get a list of DS pods
	pods, err := dsc.client.CoreV1().Pods(dsc.Namespace).List(metav1.ListOptions{
		IncludeUninitialized: true,
		LabelSelector:        "app=" + dsc.DaemonSetName + ",source=kuberhealthy",
	})
	if err != nil {
		return nodesMissingDSPods, err
	}

	// populate a node status map. default status is "false", meaning there is
	// not a pod deployed to that node.  We are only adding nodes that tolerate
	// our list of dsc.tolerations
	nodeStatuses := make(map[string]bool)
	for _, n := range nodes.Items {
		if taintsAreTolerated(n.Spec.Taints, dsc.tolerations) {
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
				if taintsAreTolerated(node.Spec.Taints, dsc.tolerations) {
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

// Deploy creates a daemon set
func (dsc *Checker) deploy() error {
	//Generate the spec for the DS that we are about to deploy
	dsc.generateDaemonSetSpec()
	//Generate DS client and create the set with the template we just generated
	daemonSetClient := dsc.getDaemonSetClient()
	_, err := daemonSetClient.Create(dsc.DaemonSet)
	dsc.DaemonSetDeployed = true
	return err
}

// remove removes a specified ds from a namespaces
func (dsc *Checker) remove() error {

	// confirm the count we are removing before issuing a delete
	podsClient := dsc.client.CoreV1().Pods(dsc.Namespace)
	pods, err := podsClient.List(metav1.ListOptions{
		IncludeUninitialized: true,
		LabelSelector:        "app=" + dsc.DaemonSetName + ",source=kuberhealthy",
	})
	if err != nil {
		return err
	}
	log.Infoln(dsc.Name(), "removing", len(pods.Items), "daemonset pods")

	// delete the daemonset
	log.Infoln(dsc.Name(), "removing daemonset")
	daemonSetClient := dsc.getDaemonSetClient()
	err = daemonSetClient.Delete(dsc.DaemonSetName, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	// issue a delete to every pod. removing the DS alone does not ensure all
	// pods are removed
	log.Infoln(dsc.Name(), "removing daemonset pods")
	err = podsClient.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
		IncludeUninitialized: true,
		LabelSelector:        "app=" + dsc.DaemonSetName + ",source=kuberhealthy",
	})
	if err != nil {
		return err
	}
	dsc.DaemonSetDeployed = false
	return nil
}

// waitForPodRemoval waits for the daemonset to finish removal
func (dsc *Checker) waitForPodRemoval(ctx context.Context) error {

	podsClient := dsc.client.CoreV1().Pods(dsc.Namespace)

	// as a fix for kuberhealthy #74 we routinely ask the pods to remove.
	// this is a workaround for a race in kubernetes that sometimes leaves
	// daemonset pods in a 'Ready' state after the daemonset has been deleted
	deleteTicker := time.NewTicker(time.Second * 30)

	// loop until all our daemonset pods are deleted
	for {
		// check for our context to expire to break the loop
		ctxErr := ctx.Err()
		if ctxErr != nil {
			return ctxErr
		}

		pods, err := podsClient.List(metav1.ListOptions{
			IncludeUninitialized: true,
			LabelSelector:        "app=" + dsc.DaemonSetName + ",source=kuberhealthy",
		})
		if err != nil {
			return err
		}

		log.Infoln(dsc.Name(), "using LabelSelector: app="+dsc.DaemonSetName+",source=kuberhealthy")

		// if the delete ticker has ticked, then issue a repeat request
		// for pods to be deleted.  See kuberhealthy issue #74
		select {
		case <-deleteTicker.C:
			log.Infoln(dsc.Name(), "Re-issuing a pod delete command for daemonset checkers.")
			err = podsClient.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
				IncludeUninitialized: true,
				LabelSelector:        "app=" + dsc.DaemonSetName + ",source=kuberhealthy",
			})
			if err != nil {
				return err
			}
		default:
		}

		// check all pods for any kuberhealthy test daemonset pods that still exist
		log.Infoln(dsc.Name(), "Daemonset check waiting for", len(pods.Items), "pods to delete")
		for _, p := range pods.Items {
			log.Infoln(dsc.Name(), "Test daemonset pod is still removing:", p.Namespace, p.Name, "on node", p.Spec.NodeName)
		}

		if len(pods.Items) == 0 {
			log.Infoln(dsc.Name(), "Test daemonset has finished removing pods")
			return nil
		}
		time.Sleep(time.Second * 1)
	}

}

// getDaemonSetClient returns a daemon set client, useful for interacting with daemonsets
func (dsc *Checker) getDaemonSetClient() v1beta1.DaemonSetInterface {
	return dsc.client.ExtensionsV1beta1().DaemonSets(dsc.Namespace)
}
