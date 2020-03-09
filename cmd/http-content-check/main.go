package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	checkclient "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
)

// TargetURL retrieves URL that will be used to search for string in response body
var TargetURL = os.Getenv("TARGET_URL")

// TargetString is the string that will be searched for in the server response body
var TargetString = os.Getenv("TARGET_STRING")

// TimeoutDur is user requested timeout duration for specified URL
var TimeoutDur = os.Getenv("TIMEOUT_DURATION")

// reportErrorAndStop reports to kuberhealthy of error and exits program when called
func reportErrorAndStop(s string) {
	log.Println("attempting to report error to kuberhealthy:", s)
	err := checkclient.ReportFailure([]string{s})
	if err != nil {
		log.Println("failed to report to kuberhealthy servers:", err)
		os.Exit(1)
	}
	log.Println("successfully reported error to kuberhealthy servers")
	os.Exit(0)
}

func main() {
	// check to make sure URL is provided
	if TargetURL == "" {
		reportErrorAndStop("No URL provided in YAML")
	}

	//check to make sure string is provided
	if TargetString == "" {
		reportErrorAndStop("No string provided in YAML")
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
	log.Println("successfully reported to kuberhealthy servers")

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
