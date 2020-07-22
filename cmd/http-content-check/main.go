package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
	"k8s.io/client-go/kubernetes"

	checkclient "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
)

var (
	// TargetURL retrieves URL that will be used to search for string in response body
	TargetURL = os.Getenv("TARGET_URL")

	// TargetString is the string that will be searched for in the server response body
	TargetString = os.Getenv("TARGET_STRING")

	// TimeoutDur is user requested timeout duration for specified URL
	TimeoutDur = os.Getenv("TIMEOUT_DURATION")

	// the global kubernetes client
	kubernetesClient *kubernetes.Clientset
)

func init() {
	// set debug mode for nodeCheck pkg
	nodeCheck.EnableDebugOutput()

	// check to make sure URL is provided
	if TargetURL == "" {
		reportErrorAndStop("No URL provided in YAML")
	}

	//check to make sure string is provided
	if TargetString == "" {
		reportErrorAndStop("No string provided in YAML")
	}
}

func main() {

	checkTimeLimit := time.Minute * 1
	ctx, ctxCancel := context.WithTimeout(context.Background(), checkTimeLimit)

	kubernetesClient, err := kubeClient.Create("")
	if err != nil {
		ctxCancel()
		reportErrorAndStop("Error creating kubeClient with error" + err.Error())
	}

	err = nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		ctxCancel()
		reportErrorAndStop("Error waiting for kuberhealthy endpoint to be contactable by checker pod with error:" + err.Error())
	}

	err = nodeCheck.WaitForKubeProxy(ctx, kubernetesClient, "kuberhealthy")
	if err != nil {
		reportErrorAndStop("Error waiting for kube proxy to be ready and running on the node with error:" + err.Error())
	}

	// attempt to fetch URL content and fail if we cannot
	userURLstring, err := getURLContent(TargetURL)
	if err != nil {
		reportErrorAndStop(err.Error())
	}

	// if we cannot find the content string the test has failed
	if !findStringInContent(userURLstring, TargetString) {
		reportErrorAndStop("could not find string in content")
	}

	// if nothing has failed the test is succesfull
	err = checkclient.ReportSuccess()
	if err != nil {
		log.Println("failed to report success", err)
		os.Exit(1)
	}
	log.Println("Successfully reported to Kuberhealthy")
}

// getURLContent retrieves bytes and error from URL
func getURLContent(url string) ([]byte, error) {
	dur, err := time.ParseDuration(TimeoutDur)
	if err != nil {
		return []byte{}, err
	}
	client := http.Client{Timeout: dur}
	resp, err := client.Get(url)
	if err != nil {
		return []byte{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err

	}
	defer resp.Body.Close()
	return body, err

}

// findStringInContent parses through URL bytes for specified string and returns bool
func findStringInContent(b []byte, s string) bool {

	stringbody := string(b)
	if strings.Contains(stringbody, s) {
		return true
	}
	return false
}

// reportErrorAndStop reports to kuberhealthy of error and exits program when called
func reportErrorAndStop(s string) {
	log.Println("attempting to report error to kuberhealthy:", s)
	err := checkclient.ReportFailure([]string{s})
	if err != nil {
		log.Println("failed to report to kuberhealthy servers:", err)
		os.Exit(1)
	}
	log.Println("Successfully reported to Kuberhealthy")
	os.Exit(0)
}
