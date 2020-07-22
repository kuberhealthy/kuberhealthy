package main

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

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

	// fetches kube proxy to see if it is ready
	err = nodeCheck.WaitForKubeProxy(ctx, kubernetesClient, "kuberhealthy", "kube-system")
	if err != nil {
		log.Errorln("Error waiting for kube proxy to be ready and running on the node with error:" + err.Error())
	}

	// attempt to fetch URL content and fail if we cannot
	userURLstring, err := getURLContent(TargetURL)
	log.Infoln("Attempting to fetch content from: " + TargetURL)
	if err != nil {
		reportErrorAndStop(err.Error())
	}

	log.Infoln("Parsing content for string " + TargetString)

	// if we cannot find the content string the test has failed
	if !findStringInContent(userURLstring, TargetString) {
		reportErrorAndStop("could not find string in content")
	}

	log.Infoln("Success! Found " + TargetString + " in " + TargetURL)

	// if nothing has failed the test is succesfull
	err = checkclient.ReportSuccess()
	if err != nil {
		log.Errorln("failed to report success", err)
		os.Exit(1)
	}
	log.Infoln("Successfully reported to Kuberhealthy")
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
	log.Infoln("attempting to report error to kuberhealthy:", s)
	err := checkclient.ReportFailure([]string{s})
	if err != nil {
		log.Errorln("failed to report to kuberhealthy servers:", err)
		os.Exit(1)
	}
	log.Infoln("Successfully reported to Kuberhealthy")
	os.Exit(0)
}
