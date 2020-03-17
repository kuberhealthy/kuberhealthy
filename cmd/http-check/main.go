package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
)

var (
	// HTTP endpoint to send a request to.
	checkURL = os.Getenv("CHECK_URL")
)

func init() {
	// Check that the URL environment variable is valid.
	if len(checkURL) == 0 {
		log.Fatalln("Empty URL. Please update your CHECK_URL environment variable.")
	}

	// If the URL does not begin with HTTP, exit.
	if !strings.HasPrefix(checkURL, "http") {
		log.Fatalln("Given URL does not declare a supported protocol. (http | https)")
	}
}

func main() {
	log.Infoln("Beginning check.")

	// Make a GET request.
	r, err := http.Get(checkURL)
	if err != nil {
		reportErr := fmt.Errorf("error occurred performing a " + http.MethodGet + " to " + checkURL + ": " + err.Error())
		log.Errorln(reportErr.Error())
		err = kh.ReportFailure([]string{reportErr.Error()})
		if err != nil {
			log.Fatalln("error when reporting to kuberhealthy:", err.Error())
		}
		os.Exit(0)
	}

	// Check if the response status code is a 200.
	if r.StatusCode == http.StatusOK {
		log.Infoln("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
		kh.ReportSuccess()
		os.Exit(0)
	}

	reportErr := fmt.Errorf("unable to retrieve a " + strconv.Itoa(http.StatusOK) + " from " + checkURL + " got a " + strconv.Itoa(r.StatusCode) + " instead")
	log.Errorln(reportErr.Error())
	kh.ReportFailure([]string{reportErr.Error()})

	os.Exit(0)
}
