// Copyright 2018 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package daemonSetCheck contains a Kuberhealthy check for the ability to roll out
// a daemonset to a cluster.  Includes validation of cleanup as well.  This
// check provides a high level of confidence that the cluster is operating
// normally.
package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	log "github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/util"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
)

const daemonSetBaseName = "daemonset"
const defaultDSCheckTimeout = "10m"
const defaultUser = int64(1000)

var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var Namespace string
var Timeout time.Duration

// DSPauseContainerImageOverride specifies the sleep image we will use on the daemonset checker
var DSPauseContainerImageOverride string // specify an alternate location for the DSC pause container - see #114
var CheckRunTime int64                   // use this to compare and find rogue daemonsets or pods

// Checker implements a KuberhealthyCheck for daemonset
// deployment and teardown checking.
type Checker struct {
	Namespace           string
	DaemonSet           *appsv1.DaemonSet
	shuttingDown        bool
	DaemonSetDeployed   bool
	DaemonSetName       string
	PauseContainerImage string
	hostname            string
	Tolerations         []apiv1.Toleration
	client              *kubernetes.Clientset
	cancelFunc          context.CancelFunc // used to cancel things in-flight
	ctx                 context.Context    // a context used for tracking check runs
	OwnerReference      []metav1.OwnerReference
}

func init() {
	// Set global vars from env variables
	Namespace = os.Getenv("POD_NAMESPACE")
	if len(Namespace) == 0 {
		log.Errorln("ERROR: The POD_NAMESPACE environment variable has not been set.")
		return
	}

	dsCheckTimeout := os.Getenv("CHECK_POD_TIMEOUT")
	if len(dsCheckTimeout) == 0 {
		log.Infoln("CHECK_POD_TIMEOUT environment variable has not been set. Using default Daemonset Checker timeout", defaultDSCheckTimeout)
		dsCheckTimeout = defaultDSCheckTimeout
	}

	DSPauseContainerImageOverride = os.Getenv("PAUSE_CONTAINER_IMAGE")

	var err error
	Timeout, err = time.ParseDuration(dsCheckTimeout)
	if err != nil {
		log.Errorln("Error parsing timeout for check", dsCheckTimeout, err)
		return
	}

	CheckRunTime = time.Now().Unix()
}

func main() {
	client, err := kubeClient.Create(KubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	ds, err := New()
	if err != nil {
		log.Fatalln("unable to create daemonset checker:", err)
	}

	ds.client = client

	// Get ownerReference
	ownerReference, err := util.GetOwnerRef(ds.client, ds.Namespace)
	if err != nil {
		log.Errorln("Failed to get ownerReference for daemonset check")
	}
	ds.OwnerReference = ownerReference

	// start listening for shutdown interrupts
	go ds.listenForInterrupts()

	// Create a context for this cleanup
	ds.ctx, ds.cancelFunc = context.WithCancel(context.Background())

	// Cleanup any daemonsets and associated pods from this check that should not exist right now
	log.Infoln("Deleting any rogue daemonsets or daemonset pods before deploying the daemonset check")
	err = ds.cleanupOrphans()
	if err != nil {
		log.Errorln("Error cleaning up rogue daemonsets or daemonset pods:", err)
	}

	// init a timeout for this whole deletion of daemonsets
	log.Infoln("Timeout set to", Timeout.String())
	timeoutChan := time.After(Timeout)

	// waiting for all daemonsets to be gone...
	log.Infoln("Waiting for all rogue daemonsets or daemonset pods to clean up")
	select {
	case <-timeoutChan:
		log.Infoln("timed out")
		ds.cancel() // cancel the watch context, we have timed out
		log.Errorln("failed to see rogue daemonset or daemonset pods cleanup within timeout")
	case err = <-ds.waitForAllDaemonsetsToClear():
		if err != nil {
			ds.cancel() // cancel the watch context, we have timed out
			log.Errorln("error waiting for rogue daemonset or daemonset pods to clean up:", err)
		}
		log.Infoln("No rogue daemonsets or daemonset pods exist.")
	}

	// allow the user to override the image used by the DSC - see #114
	ds.overrideDSPauseContainerImage()

	log.Infoln("Enabling daemonset checker")

	err = ds.Run(client)
	if err != nil {
		log.Errorln("Error running check:", ds.Name(), "in namespace", ds.CheckNamespace()+":", err)
		os.Exit(1)
	}
	log.Debugln("Done running check:", ds.Name(), "in namespace", ds.CheckNamespace())
}

// New creates a new Checker object
func New() (*Checker, error) {

	hostname := getHostname()
	var tolerations []apiv1.Toleration

	testDS := Checker{
		Namespace:           Namespace,
		DaemonSetName:       daemonSetBaseName + "-" + hostname + "-" + strconv.Itoa(int(CheckRunTime)),
		hostname:            hostname,
		PauseContainerImage: "gcr.io/google-containers/pause:3.1",
		Tolerations:         tolerations,
	}

	return &testDS, nil
}

// cancel cancels the context of this checker to shut things down gracefully
func (dsc *Checker) cancel() {
	if dsc.cancelFunc == nil {
		return
	}
	dsc.cancelFunc()
}

func (dsc *Checker) overrideDSPauseContainerImage() {
	// allow the user to override the image used by the DSC - see #114
	if len(DSPauseContainerImageOverride) > 0 {
		log.Info("Setting DS pause container override image to:", DSPauseContainerImageOverride)
		dsc.PauseContainerImage = DSPauseContainerImageOverride
	}
}

// generateDaemonSetSpec generates a daemon set spec to deploy into the cluster
func (dsc *Checker) generateDaemonSetSpec() {

	checkRunTime := strconv.Itoa(int(CheckRunTime))
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
		dsc.Tolerations, err = findAllUniqueTolerations(dsc.client)
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
			OwnerReferences: dsc.OwnerReference,
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
					OwnerReferences: dsc.OwnerReference,
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
// Default is 10 minutes if the CHECK_POD_TIMEOUT env var is not set in the Kuberhealthy external check spec
func (dsc *Checker) Timeout() time.Duration {
	return Timeout
}

// Shutdown signals the DS to begin a cleanup
func (dsc *Checker) Shutdown(sdDoneChan chan error) {
	dsc.shuttingDown = true

	var err error
	// if the ds is deployed, delete it
	if dsc.DaemonSetDeployed {
		err = dsc.remove()
		if err != nil {
			log.Infoln("Failed to remove", dsc.DaemonSetName)
		}
		err = dsc.waitForPodRemoval()
	}

	log.Infoln(dsc.Name(), "Daemonset "+dsc.DaemonSetName+" ready for shutdown.")
	sdDoneChan <- err
}

// listenForInterrupts watches for termination signals and acts on them
func (dsc *Checker) listenForInterrupts() {
	// Make neccessary shutdown channels (signal channel and done channel)
	sigChan := make(chan os.Signal, 5)
	doneChan := make(chan error, 5)

	terminationGracePeriod := time.Minute * 5

	signal.Notify(sigChan, os.Interrupt, os.Kill)
	<-sigChan
	log.Infoln("Shutting down...")
	go dsc.Shutdown(doneChan)
	// wait for checks to be done shutting down before exiting
	select {
	case err := <-doneChan:
		if err != nil {
			log.Errorln("Error waiting for pod removal during shut down")
			os.Exit(1)
		}
		log.Infoln("Shutdown gracefully completed!")
		os.Exit(0)
	case <-sigChan:
		log.Warningln("Shutdown forced from multiple interrupts!")
		os.Exit(1)
	case <-time.After(terminationGracePeriod):
		log.Errorln("Shutdown took too long.  Shutting down forcefully!")
		os.Exit(2)
	}
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

// ParseTolerationOverride parses a list of taints and returns them as a toleration object
func (dsc *Checker) ParseTolerationOverride(taints []string) (tolerations []apiv1.Toleration, err error) {
	for _, t := range taints {
		s := strings.Split(t, ",")
		if len(s) != 3 {
			return []apiv1.Toleration{}, errors.New("Unable to parse the passed in taint overrides - are they in the correct format?")
		}
		tolerations = append(tolerations, apiv1.Toleration{
			Key:    s[0],
			Value:  s[1],
			Effect: apiv1.TaintEffect(s[2]),
		})
	}
	return tolerations, err
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
// creating instance is gone and ensures thay are not pods from an older run.
// Sometimes removing daemonsets isnt enough to clean up orphaned pods.
func (dsc *Checker) cleanupOrphanedPods() error {
	pods, err := dsc.getAllPods()
	if err != nil {
		log.Errorln("Error fetching pods:", err)
		return err
	}

	// loop on all the daemonsets and ensure that daemonset's creating pod exists and that the pods are not from an older run
	// if the creating pod does not exist, then we delete the daemonset.
	for _, p := range pods {
		log.Infoln("Checking if pod is orphaned:", p.Name, "creatingInstance:", p.Labels["creatingInstance"], "checkRunTime:", CheckRunTime)

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

		// if the owning kuberhealthy pod of the DS does not exist, then we delete the daemonset
		if !exists {
			log.Infoln("Removing orphaned pod", p.Name, "because kuberhealthy ds", creatingDSInstance, "does not exist")
			err := dsc.deletePod(p.Name)
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

		if podCheckRunTime < CheckRunTime {
			log.Warningln("Pod:", p.Name, "has an older checkRunTime than the current daemonset running. This is a rogue pod, removing now.")
			err := dsc.deletePod(p.Name)
			if err != nil {
				log.Warningln("error when removing rogue pod:", p.Name+": ", err)
			}
			continue
		}
	}
	return nil
}

// cleanupOrphanedDaemonsets cleans up daemonsets that should not exist based on their
// creatingInstance label and ensures they are not daemonsets from an older run
func (dsc *Checker) cleanupOrphanedDaemonsets() error {

	daemonSets, err := dsc.getAllDaemonsets()
	if err != nil {
		log.Errorln("Error fetching daemonsets for cleanup:", err)
		return err
	}

	// loop on all the daemonsets and ensure that daemonset's creating pod exists and that the daemonsets are not from an older run.
	// if the creating pod does not exist, then we delete the daemonset.
	for _, ds := range daemonSets {
		log.Infoln("Checking if daemonset is orphaned:", ds.Name, "creatingInstance:", ds.Labels["creatingInstance"], "checkRunTime:", CheckRunTime)

		// fetch the creatingInstance label
		creatingInstance := ds.Labels["creatingInstance"]

		// if there isn't a creatingInstance label, we assume its an old generation and remove it.
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

		// if the owning kuberhealthy pod of the DS does not exist, then we delete the daemonset
		if !exists {
			log.Infoln("Removing orphaned daemonset", ds.Name, "because creating kuberhealthy instance", creatingInstance, "does not exist")
			err := dsc.deleteDS(ds.Name)
			if err != nil {
				log.Warningln("error when removing orphaned daemonset", ds.Name+": ", err)
				return err
			}
		}

		// Check that the daemonset isn't from an older run
		dsCheckRunTime, err := strconv.ParseInt(ds.Labels["checkRunTime"], 10, 64)
		if err != nil {
			log.Errorln("Error converting ds checkRunTime:", dsCheckRunTime, "label to int:", err)
		}

		if dsCheckRunTime < CheckRunTime {
			log.Warningln("Daemonset:", ds.Name, "has an older checkRunTime than the current daemonset running. This is a rogue daemonset, removing now.")
			err := dsc.deleteDS(ds.Name)
			if err != nil {
				log.Warningln("error when removing rogue daemonset:", ds.Name+": ", err)
			}
			continue
		}
	}
	return nil
}

// deleteDS deletes the DS with the specified name
func (dsc *Checker) deleteDS(dsName string) error {

	// confirm the count we are removing before issuing a delete
	podsClient := dsc.client.CoreV1().Pods(dsc.Namespace)
	pods, err := podsClient.List(metav1.ListOptions{
		LabelSelector: "app=" + dsName + ",source=kuberhealthy,khcheck=daemonset",
	})
	if err != nil {
		return err
	}
	log.Infoln(dsc.Name(), "removing", len(pods.Items), "daemonset pods")

	// delete the daemonset
	log.Infoln(dsc.Name(), "removing daemonset:", dsName)
	daemonSetClient := dsc.getDaemonSetClient()
	err = daemonSetClient.Delete(dsName, &metav1.DeleteOptions{})
	if err != nil {
		log.Error("Failed to delete daemonset:", dsName, err)
		return err
	}

	// issue a delete to every pod. removing the DS alone does not ensure all
	// pods are removed
	log.Infoln(dsc.Name(), "removing daemonset pods")
	err = podsClient.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: "app=" + dsName + ",source=kuberhealthy,khcheck=daemonset",
	})
	if err != nil {
		log.Error("Failed to delete daemonset pods:", err)
		return err
	}
	return nil

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
		podList, err := dsc.client.CoreV1().Pods(dsc.Namespace).List(metav1.ListOptions{
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

// getAllDaemonsets fetches all daemonsets in the namespace, for all
// instances of kuberhealthy
func (dsc *Checker) getAllDaemonsets() ([]appsv1.DaemonSet, error) {

	var allDS []appsv1.DaemonSet
	var cont string
	var err error

	// fetch the ds objects created by kuberhealthy
	for {
		var dsList *appsv1.DaemonSetList
		dsClient := dsc.getDaemonSetClient()
		dsList, err = dsClient.List(metav1.ListOptions{
			LabelSelector: "source=kuberhealthy,khcheck=daemonset",
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
	dsc.ctx, dsc.cancelFunc = context.WithCancel(context.Background())

	doneChan := make(chan error)

	dsc.client = client

	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := dsc.doChecks()
		doneChan <- err
	}(doneChan)

	var err error
	// wait for either a timeout or job completion
	select {
	case <-time.After(dsc.Interval()):
		// The check has timed out because its time to run again
		dsc.cancelFunc() // cancel context
		errorMessage := "Failed to complete checks for " + dsc.Name() + " in time!  Next run came up but check was still running."
		//log.Errorln(dsc.Name(), errorMessage)
		err = checkclient.ReportFailure([]string{errorMessage})
		if err != nil {
			log.Println("Error reporting failure to Kuberhealthy servers:", err)
			return err
		}
		log.Println("Successfully reported failure to Kuberhealthy servers")
	case <-time.After(dsc.Timeout()):
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
	go dsc.cleanupOrphans()

	return nil
}

// waitForAllDaemonsetsToClear
func (dsc *Checker) waitForAllDaemonsetsToClear() chan error {
	log.Infoln("waiting for all daemonsets to clear")

	// make the output channel we will return and close it whenever we are done
	outChan := make(chan error, 2)

	go func() {
		// watch events and return when the pod is in state running
		for {
			log.Debugln("Waiting for daemonset", dsc.Name(), "to clear...")

			// wait between requests
			time.Sleep(time.Second * 5)

			// if the context is canceled, we stop
			select {
			case <-dsc.ctx.Done():
				outChan <- errors.New("waiting for daemonset to clear was aborted by context cancellation")
				return
			default:
			}

			// fetch the pod by name
			dsList, err := dsc.getAllDaemonsets()
			if err != nil {
				outChan <- err
				return
			}
			if len(dsList) == 0 {
				log.Info("all daemonsets cleared")
				outChan <- nil
				return
			}
		}
	}()

	return outChan
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
func (dsc *Checker) doDeploy() error {

	// create DS
	dsc.DaemonSetDeployed = true
	err := dsc.deploy()
	if err != nil {
		log.Error("Something went wrong with daemonset deployment, cleaning things up...", err)
		err2 := dsc.doRemove()
		if err2 != nil {
			log.Error("Something went wrong when removing the deployment after a deployment error:", err2)
		}
		return err
	}

	// wait for ds pods to be created
	err = dsc.waitForPodsToComeOnline()
	return err
}

// doRemove remotes the daemonset from the cluster
func (dsc *Checker) doRemove() error {
	// delete ds
	err := dsc.remove()
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
	dsc.DaemonSetDeployed = true
	return err
}

// waitForPodsToComeOnline blocks until all pods of the daemonset are deployed and online
func (dsc *Checker) waitForPodsToComeOnline() error {

	// counter for DS status check below
	var counter int
	var nodesMissingDSPod []string

	for {
		ctxErr := dsc.ctx.Err()
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

// Deploy creates a daemon set
func (dsc *Checker) deploy() error {
	//Generate the spec for the DS that we are about to deploy
	dsc.generateDaemonSetSpec()
	//Generate DS client and create the set with the template we just generated
	daemonSetClient := dsc.getDaemonSetClient()
	_, err := daemonSetClient.Create(dsc.DaemonSet)
	if err != nil {
		log.Error("Failed to create daemon set:", err)
	}
	dsc.DaemonSetDeployed = true
	return err
}

// remove removes a specified ds from a namespaces
func (dsc *Checker) remove() error {

	// confirm the count we are removing before issuing a delete
	podsClient := dsc.client.CoreV1().Pods(dsc.Namespace)
	pods, err := podsClient.List(metav1.ListOptions{
		LabelSelector: "app=" + dsc.DaemonSetName + ",source=kuberhealthy,khcheck=daemonset",
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
		log.Error("Failed to delete daemonset:", err)
		return err
	}

	// issue a delete to every pod. removing the DS alone does not ensure all
	// pods are removed
	log.Infoln(dsc.Name(), "removing daemonset pods")
	err = podsClient.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: "app=" + dsc.DaemonSetName + ",source=kuberhealthy,khcheck=daemonset",
	})
	if err != nil {
		log.Error("Failed to delete daemonset pods:", err)
		return err
	}
	dsc.DaemonSetDeployed = false
	return nil
}

// waitForDSRemoval waits for the daemonset to be removed before returning
func (dsc *Checker) waitForDSRemoval() error {
	// repeatedly fetch the DS until it goes away
	for {
		// check for our context to expire to break the loop
		ctxErr := dsc.ctx.Err()
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

// waitForPodRemoval waits for the daemonset to finish removal
func (dsc *Checker) waitForPodRemoval() error {

	podsClient := dsc.client.CoreV1().Pods(dsc.Namespace)

	// as a fix for kuberhealthy #74 we routinely ask the pods to remove.
	// this is a workaround for a race in kubernetes that sometimes leaves
	// daemonset pods in a 'Ready' state after the daemonset has been deleted
	deleteTicker := time.NewTicker(time.Second * 30)

	// loop until all our daemonset pods are deleted
	for {
		// check for our context to expire to break the loop
		ctxErr := dsc.ctx.Err()
		if ctxErr != nil {
			return ctxErr
		}

		pods, err := podsClient.List(metav1.ListOptions{
			LabelSelector: "app=" + dsc.DaemonSetName + ",source=kuberhealthy,khcheck=daemonset",
		})
		if err != nil {
			return err
		}

		log.Infoln(dsc.Name(), "using LabelSelector: app="+dsc.DaemonSetName+",source=kuberhealthy,khcheck=daemonset")

		// if the delete ticker has ticked, then issue a repeat request
		// for pods to be deleted.  See kuberhealthy issue #74
		select {
		case <-deleteTicker.C:
			log.Infoln(dsc.Name(), "Re-issuing a pod delete command for daemonset checkers.")
			err = podsClient.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: "app=" + dsc.DaemonSetName + ",source=kuberhealthy,khcheck=daemonset",
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
func (dsc *Checker) getDaemonSetClient() v1.DaemonSetInterface {
	log.Debug("Creating Daemonset client.")
	return dsc.client.AppsV1().DaemonSets(dsc.Namespace)
}
