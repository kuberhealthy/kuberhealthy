package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	apiv1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

)

var (
	// K8s config file for the client
	kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

	// Namespace the check daemonset will be created in [default = kuberhealthy]
	checkNamespaceEnv = os.Getenv("POD_NAMESPACE")
	checkNamespace    string

	// Timeout set to ensure the cleanup of rogue daemonsets or daemonset pods after the check has finished within this timeout
	checkPodTimeoutEnv = os.Getenv("CHECK_POD_TIMEOUT")
	checkPodTimeout    time.Duration
	checkPodTimeoutChan = time.After(checkPodTimeout)

	// DSPauseContainerImageOverride specifies the sleep image we will use on the daemonset checker
	dsPauseContainerImageEnv = os.Getenv("PAUSE_CONTAINER_IMAGE")
	dsPauseContainerImage string // specify an alternate location for the DSC pause container - see #114

	// Minutes allowed for the shutdown process to complete
	shutdownGracePeriodEnv = os.Getenv("SHUTDOWN_GRACE_PERIOD")
	shutdownGracePeriod    time.Duration

	// Check daemonset name
	checkDSNameEnv = os.Getenv("CHECK_DAEMONSET_NAME")
	checkDSName    string

	// Check time limit from injected env variable KH_CHECK_RUN_DEADLINE
	checkTimeLimit time.Duration

	// Time object used for the check.
	now time.Time

	ctx       context.Context
	ctxCancel context.CancelFunc

	// Interrupt signal channels.
	signalChan chan os.Signal
	doneChan   chan error

	// K8s client used for the check.
	client *kubernetes.Clientset
)

const (
	// Default k8s manifest resource names.
	defaultCheckDSName = "daemonset"
	// Default namespace daemonset check will be performed in
	defaultCheckNamespace = "kuberhealthy"
	// Default check pod timeout for daemonset check
	defaultCheckPodTimeout = time.Duration(time.Minute*12)
	// Default pause container image used for the daemonset check
	defaultDSPauseContainerImage = "gcr.io/google-containers/pause:3.1"
	// Default shutdown termination grace period
	defaultShutdownGracePeriod = time.Duration(time.Minute * 5) // grace period for the check to shutdown after receiving a shutdown signal
	// Default daemonset check time limit
	defaultCheckTimeLimit = time.Duration(time.Minute * 15)

	// Default user
	defaultUser = int64(1000)

)

// Checker implements a KuberhealthyCheck for daemonset
// deployment and teardown checking.
type Checker struct {
	DaemonSet           *appsv1.DaemonSet
	shuttingDown        bool
	DaemonSetDeployed   bool
	DaemonSetName       string
	PauseContainerImage string
	hostname            string
	Tolerations         []apiv1.Toleration
	//client              *kubernetes.Clientset
	//cancelFunc          context.CancelFunc // used to cancel things in-flight
	//ctx                 context.Context    // a context used for tracking check runs
}

func init() {

	// Parse all incoming input environment variables and crash if an error occurs
	// during parsing process.
	parseInputValues()

	// Allocate channels.
	signalChan = make(chan os.Signal, 5)
	doneChan = make(chan error, 5)
}

func main() {
	// Create a timestamp reference for the deployment;
	// also to reference against deployments that should be cleaned up.
	now = time.Now()
	log.Debugln("Allowing this check", checkTimeLimit, "to finish.")
	ctx, ctxCancel = context.WithTimeout(context.Background(), checkTimeLimit)

	// Create a kubernetes client.
	var err error
	client, err = kubeClient.Create(kubeConfigFile)
	if err != nil {
		errorMessage := "failed to create a kubernetes client with error: " + err.Error()
		reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + errorMessage})
		return
	}
	log.Infoln("Kubernetes client created.")

	// Instantiate new daemonset check object
	ds := New()

	// Start listening to interrupts.
	go ds.listenForInterrupts()

	// Catch panics.
	var r interface{}
	defer func() {
		r = recover()
		if r != nil {
			log.Infoln("Recovered panic:", r)
			reportErrorsToKuberhealthy([]string{"kuberhealthy/daemonset: " + r.(string)})
		}
	}()

	// Start daemonset check.
	ds.runDaemonsetCheck()
}

// New creates a new daemonset checker object
func New() *Checker {

	hostname := getHostname()
	var tolerations []apiv1.Toleration

	dsObject := Checker{
		DaemonSetName:       checkDSName + "-" + hostname + "-" + strconv.Itoa(int(now.Unix())),
		hostname:            hostname,
		PauseContainerImage: dsPauseContainerImage,
		Tolerations:         tolerations,
	}

	return &dsObject
}

// listenForInterrupts watches the signal and done channels for termination.
func (dsc *Checker) listenForInterrupts() {

	// Relay incoming OS interrupt signals to the signalChan
	signal.Notify(signalChan, os.Interrupt, os.Kill)
	sig :=<-signalChan
	log.Infoln("Received an interrupt signal from the signal channel.")
	log.Debugln("Signal received was:", sig.String())

	go dsc.shutdown(doneChan)
	// wait for checks to be done shutting down before exiting
	select {
	case err := <-doneChan:
		if err != nil {
			log.Errorln("Error waiting for pod removal during shut down:", err)
			os.Exit(1)
		}
		log.Infoln("Shutdown gracefully completed!")
		os.Exit(0)
	case sig = <-signalChan:
		log.Warningln("Shutdown forced from multiple interrupts!")
		os.Exit(1)
	case <-time.After(time.Duration(shutdownGracePeriod)):
		log.Errorln("Shutdown took too long. Shutting down forcefully!")
		os.Exit(2)
	}
}

// Shutdown signals the DS to begin a cleanup
func (dsc *Checker) shutdown(sdDoneChan chan error) {
	dsc.shuttingDown = true

	var err error
	// if the ds is deployed, delete it
	if dsc.DaemonSetDeployed {
		err = deleteDS(dsc.DaemonSetName)
		if err != nil {
			log.Errorln("Failed to remove", dsc.DaemonSetName)
		}
		dsc.DaemonSetDeployed = false
		err = <- dsc.waitForPodRemoval()
	}

	log.Infoln("Daemonset", dsc.DaemonSetName, "ready for shutdown.")
	sdDoneChan <- err
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
	log.Infoln("DaemonsetChecker deleting daemonset:", dsName)
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
func (dsc *Checker) waitForPodRemoval() chan error {
	// as a fix for kuberhealthy #74 we routinely ask the pods to remove.
	// this is a workaround for a race in kubernetes that sometimes leaves
	// daemonset pods in a 'Ready' state after the daemonset has been deleted
	deleteTicker := time.NewTicker(time.Second * 30)

	// make the output channel we will return and close it whenever we are done
	outChan := make(chan error)

	timePassed := now.Sub(time.Now())
	checkPodTimeout = checkPodTimeout + timePassed
	log.Infoln("Timeout set:", checkPodTimeout.String(), "for daemonset pods removal.")

	// loop until all daemonset pods are deleted
	go func() {
		for {
			// check for our context to expire to break the loop
			ctxErr := ctx.Err()
			if ctxErr != nil {
				outChan <- ctxErr
				return
			}

			pods, err := listDSPods(dsc.DaemonSetName)
			if err != nil {
				outChan <- err
				return
			}

			log.Infoln("DaemonsetChecker using LabelSelector: app="+dsc.DaemonSetName+",source=kuberhealthy,khcheck=daemonset to remove ds pods")

			// If the delete ticker has ticked, then issue a repeat request for pods to be deleted.
			// See kuberhealthy issue #74
			select {
			case <-deleteTicker.C:
				log.Infoln("DaemonsetChecker re-issuing a pod delete command for daemonset checkers.")
				err = deleteDSPods(dsc.DaemonSetName)
				if err != nil {
					outChan <- err
					return
				}
			case <-checkPodTimeoutChan:
				nodeList := getDSPodsNodeList(pods)
				errorMessage := "Reached check pod timeout: " + "3s" + " waiting for daemonset pods removal. " +
					"Failed to remove DS pods from nodes: " + formatNodes(nodeList)
				log.Errorln(errorMessage)
				outChan <- err
				return
			default:
			}

			// Check all pods for any kuberhealthy test daemonset pods that still exist
			log.Infoln("DaemonsetChecker waiting for", len(pods.Items), "pods to delete")
			for _, p := range pods.Items {
				log.Infoln("DaemonsetChecker is still removing:", p.Namespace, p.Name, "on node", p.Spec.NodeName)
			}

			if len(pods.Items) == 0 {
				log.Infoln("DaemonsetChecker has finished removing all daemonset pods")
				outChan <- nil
				return
			}
			time.Sleep(time.Second * 1)
		}
	}()
	return outChan
}

// waitForDSRemoval waits for the daemonset to be removed before returning
func (dsc *Checker) waitForDSRemoval() chan error {

	// make the output channel we will return and close it whenever we are done
	outChan := make(chan error)

	timePassed := now.Sub(time.Now())
	checkPodTimeout = checkPodTimeout + timePassed
	log.Infoln("Timeout set:", checkPodTimeout.String(), "for daemonset removal.")

	// repeatedly fetch the DS until it goes away
	go func() {
		for {
			select {
			case <- ctx.Done():
				outChan <- errors.New("Waiting for daemonset: " + dsc.DaemonSetName + " removal aborted by context cancellation.")
				return
			case <- time.After(checkPodTimeout):
				errorMessage := "Reached check pod timeout: " + checkPodTimeout.String() + " waiting for daemonset: " + dsc.DaemonSetName + " removal."
				log.Errorln(errorMessage)
				outChan <- errors.New(errorMessage)
				return
			default:
			}
			// check for our context to expire to break the loop
			ctxErr := ctx.Err()
			if ctxErr != nil {
				outChan <- ctxErr
				return
			}
			time.Sleep(time.Second / 2)
			exists, err := fetchDS(dsc.DaemonSetName)
			if err != nil {
				outChan <- err
				return
			}
			if !exists {
				outChan <- nil
				return
			}
		}
	}()
	return outChan

}

// getDaemonsetClient returns a daemonset client, useful for interacting with daemonsets
func getDSClient() v1.DaemonSetInterface {
	log.Debug("Creating Daemonset client.")
	return client.AppsV1().DaemonSets(checkNamespace)
}

// getDaemonSetClient returns a pod client, useful for interacting with pods
func getPodClient() v12.PodInterface {
	log.Debug("Creating Pod client.")
	return client.CoreV1().Pods(checkNamespace)
}

// getDaemonSetClient returns a pod client, useful for interacting with pods
func getNodeClient() v12.NodeInterface {
	log.Debug("Creating Node client.")
	return client.CoreV1().Nodes()
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

// reportErrorsToKuberhealthy reports the specified errors for this check run.
func reportErrorsToKuberhealthy(errs []string) {
	log.Errorln("Reporting errors to Kuberhealthy:", errs)
	reportToKuberhealthy(false, errs)
}

// reportOKToKuberhealthy reports that there were no errors on this check run to Kuberhealthy.
func reportOKToKuberhealthy() {
	log.Infoln("Reporting success to Kuberhealthy.")
	reportToKuberhealthy(true, []string{})
}

// reportToKuberhealthy reports the check status to Kuberhealthy.
func reportToKuberhealthy(ok bool, errs []string) {
	var err error
	if ok {
		err = kh.ReportSuccess()
		if err != nil {
			log.Fatalln("error reporting to kuberhealthy:", err.Error())
		}
		return
	}
	err = kh.ReportFailure(errs)
	if err != nil {
		log.Fatalln("error reporting to kuberhealthy:", err.Error())
	}
	return
}