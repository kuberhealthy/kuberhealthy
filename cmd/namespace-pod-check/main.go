package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	kh "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
)

var podName = "kuberhealthy-namespace-checker-pod"

func init() {
	// set debug mode for nodeCheck pkg
	nodeCheck.EnableDebugOutput()
}

func main() {
	// create context
	checkTimeLimit := time.Minute * 1
	ctx, _ := context.WithTimeout(context.Background(), checkTimeLimit)

	// create kubernetes client
	kubernetesClient, err := kubeClient.Create("")
	if err != nil {
		log.Errorln("Error creating kubeClient with error" + err.Error())
	}

	// hits kuberhealthy endpoint to see if node is ready
	err = nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		log.Errorln("Error waiting for kuberhealthy endpoint to be contactable by checker pod with error:" + err.Error())
	}

	//create namespace interface
	nsi := kubernetesClient.CoreV1().Namespaces()

	listOpts := v1.ListOptions{}

	//create namespace list
	namespaces, err := nsi.List(ctx, listOpts)

	log.Infoln("Found", len(namespaces.Items), "namespaces")

	//counter for test pods
	successfulPods := 0
	failedPods := 0

	//range over namespaces and test deployment and deletion of test pods
	for _, n := range namespaces.Items {
		log.Infoln("DEPLOYING POD IN NAMESPACE", n.Name)
		err := deployPod(ctx, n.Name, podName, kubernetesClient)
		if err != nil {
			log.Error(err)
			failedPods++
			continue
		}
		err = deletePod(ctx, n.Name, podName, kubernetesClient)
		if err != nil {
			log.Error(err)
			failedPods++
			continue
		}
		successfulPods++
	}

	if failedPods != 0 {
		reportErr := fmt.Errorf("namespace-pod-check was unable to deploy or delete test pods in " + strconv.Itoa(failedPods) + " out of " + strconv.Itoa(len(namespaces.Items)) + " namespaces")
		ReportFailureAndExit(reportErr)
	}
	log.Info("namespace-pod-check was able to succesfully deploy and delete test pods in ", successfulPods, " namespaces")
	err = kh.ReportSuccess()
	if err != nil {
		log.Fatalln("error when reporting to kuberhealthy with error:", err)
	}
	log.Infoln("Successfully reported to kuberhealthy.")
}

func deployPod(ctx context.Context, namespace string, name string, client *kubernetes.Clientset) error {

	//pod defination
	pod := getPodObject(name, namespace)

	opts := metav1.CreateOptions{}

	//create the pod in kubernetes cluster using the clientset
	pod, err := client.CoreV1().Pods(namespace).Create(ctx, pod, opts)
	if err != nil {
		reportErr := fmt.Errorf("Error deploying pod " + name + " in namespace: " + namespace + " with error " + error.Error(err))
		return reportErr
	}
	log.Infoln("Pod", name, "created successfully in namespace:", namespace)
	return nil
}

//deletePod deletes pod in namespace
func deletePod(ctx context.Context, namespace string, name string, client *kubernetes.Clientset) error {

	delOpts := v1.DeleteOptions{}
	err := client.CoreV1().Pods(namespace).Delete(ctx, name, delOpts)
	if err != nil {
		reportErr := fmt.Errorf("Error deleting pod " + name + " in namespace " + namespace + " with error " + error.Error(err))
		return reportErr
	}
	log.Infoln("Pod", name, "successfully deleted in namespace:", namespace)
	return nil
}

//getPodObject returns container to run for test
func getPodObject(name string, namespace string) *core.Pod {
	return &core.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "demo",
			},
		},
		Spec: core.PodSpec{
			Containers: []core.Container{
				{
					Name:            "busybox",
					Image:           "busybox",
					ImagePullPolicy: core.PullIfNotPresent,
					Command: []string{
						"sleep",
						"3600",
					},
				},
			},
		},
	}
}

// ReportFailureAndExit logs and reports an error to kuberhealthy and then exits the program.
// If a error occurs when reporting to kuberhealthy, the program fatals.
func ReportFailureAndExit(err error) {
	err = kh.ReportFailure([]string{err.Error()})
	if err != nil {
		log.Fatalln("error when reporting to kuberhealthy with error:", err)
	}
	log.Infoln("Succesfully reported error to kuberhealthy")
	os.Exit(0)
}
