// Package daemonSet contains a Kuberhealthy check for the ability to roll out
// a daemonset to a cluster.  Includes validation of cleanup as well.  This
// check provides a high level of confidence that the cluster is operating
// normally.
package daemonSet

import (
	"context"
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
const namespace = "kuberhealthy"

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
	client            *kubernetes.Clientset
}

// New creates a new Checker object
func New() (*Checker, error) {

	hostname := getHostname()

	terminationGracePeriod := int64(1)
	testDS := Checker{
		ErrorMessages: []string{},
		Namespace:     namespace,
		DaemonSetName: daemonSetBaseName + "-" + hostname + "-" + strconv.Itoa(int(time.Now().Unix())),
		hostname:      hostname,
	}

	testDS.DaemonSet = &betaapiv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: testDS.DaemonSetName,
			Labels: map[string]string{
				"app":              testDS.DaemonSetName,
				"source":           "kuberhealthy",
				"creatingInstance": hostname,
			},
		},
		Spec: betaapiv1.DaemonSetSpec{
			MinReadySeconds: 2,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":              testDS.DaemonSetName,
					"source":           "kuberhealthy",
					"creatingInstance": hostname,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":              testDS.DaemonSetName,
						"source":           "kuberhealthy",
						"creatingInstance": hostname,
					},
					Name: testDS.DaemonSetName,
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Tolerations: []apiv1.Toleration{
						apiv1.Toleration{
							Key:    "node-role.kubernetes.io/master",
							Effect: "NoSchedule",
						},
					},
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

	return &testDS, nil
}

// Name returns the name of this checker
func (dsc *Checker) Name() string {
	return "DaemonSetChecker"
}

// Interval returns the interval at which this check runs
func (dsc *Checker) Interval() time.Duration {
	return time.Minute * 15
}

// Timeout returns the maximum run time for this check before it times out
func (dsc *Checker) Timeout() time.Duration {
	return time.Minute * 5
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

	log.Infoln(dsc.Name(), "Daemonset "+dsc.dsName()+" ready for shutdown.")
	return nil

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
	var err error

	// fetch the pod objects created by kuberhealthy
	fetchPodList := func() {
		var podList *apiv1.PodList
		podList, err = dsc.client.Core().Pods(dsc.Namespace).List(metav1.ListOptions{
			LabelSelector: "source=kuberhealthy",
		})
		cont = podList.Continue

		// pick the items out and add them to our end results
		for _, p := range podList.Items {
			allPods = append(allPods, p)
		}
	}

	// fech the pod list
	fetchPodList()

	// while continue is set, keep fetching items
	for len(cont) > 0 {
		fetchPodList()
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
	fetchDSList := func() {
		var dsList *betaapiv1.DaemonSetList
		dsClient := dsc.getDaemonSetClient()
		dsList, err = dsClient.List(metav1.ListOptions{
			LabelSelector: "source=kuberhealthy",
		})
		cont = dsList.Continue

		// pick the items out and add them to our end results
		for _, ds := range dsList.Items {
			allDS = append(allDS, ds)
		}
	}

	// fech the ds list
	fetchDSList()

	// while continue is set, keep fetching items
	for len(cont) > 0 {
		fetchDSList()
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
	ds, err := dsClient.Get(dsc.dsName(), metav1.GetOptions{})

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
			if item.GetName() == dsc.dsName() {
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
	dsClient := dsc.getDaemonSetClient()

	// counter for DS status check below
	var counter int

	for {
		ctxErr := ctx.Err()
		if ctxErr != nil {
			return ctxErr
		}
		time.Sleep(time.Second)

		// if we need to shut down, stop waiting entirely
		if dsc.shuttingDown {
			return nil
		}
		ds, err := dsClient.Get(dsc.dsName(), metav1.GetOptions{})
		if err != nil {
			log.Warningln(dsc.Name(), "API error when fetching daemonset:", err)
		}
		// we check to see if the number of scheduled pods matches the number
		// that are in available status, but the number scheduled must be
		// more than 0
		log.Infoln(dsc.Name(), "Daemonset check waiting for pods to come up", ds.Status.NumberAvailable, "/", ds.Status.DesiredNumberScheduled)

		// We want to ensure all the DS pods are up and healthy for at least 5 seconds
		// before moving on. This is to help verify that the DS is _actually_ healthy
		// and to mitigate possible race conditions arising from deleting pods that
		// were _just_ created

		// DS must show as healthy for 5 concurrent checks separated by 1 second each
		if ds.Status.NumberAvailable == ds.Status.DesiredNumberScheduled && ds.Status.DesiredNumberScheduled > 0 {
			counter++
			if counter >= 5 {
				log.Infoln(dsc.Name(), "Daemonset "+dsc.dsName()+" done deploying pods.")
				return nil
			}
		}
		// if the DS is unhealthy during one of our checks, set the counter back to 0
		if ds.Status.NumberAvailable != ds.Status.DesiredNumberScheduled && ds.Status.DesiredNumberScheduled > 0 {
			log.Infoln(dsc.Name(), "Daemonset "+dsc.dsName()+" was ready for ", counter, " out of 5 seconds but has left the ready state. Restarting 5 second timer.")
			counter = 0
		}
	}
}

// dsName fetches the current name of the test DS
func (dsc *Checker) dsName() string {
	return dsc.DaemonSet.ObjectMeta.Name
}

// Deploy creates a daemon set
func (dsc *Checker) deploy() error {
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
	err = daemonSetClient.Delete(dsc.dsName(), &metav1.DeleteOptions{})
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
	// loop until all our daemonset pods are deleted
	podsClient := dsc.client.CoreV1().Pods(dsc.Namespace)
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
