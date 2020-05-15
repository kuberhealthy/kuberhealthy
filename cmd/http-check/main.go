package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
)

var (
	// Environment Variables fetched from spec file
	checkURL = os.Getenv("CHECK_URL")
	count    = os.Getenv("COUNT")
	seconds  = os.Getenv("SECONDS")
)

func init() {
	// Check that the URL environment variable is valid.
	if len(checkURL) == 0 {
		log.Fatalln("Empty URL. Please update your CHECK_URL environment variable.")
	}

	// Check that the COUNT environment variable is valid.
	if len(count) == 0 {
		log.Fatalln("Empty count value. Please update your COUNT environemnt variable.")
	}

	// Check that the SECONDS environment variable is valid.
	if len(seconds) == 0 {
		log.Fatalln("Empty seconds value. Please update your SECONDS environemnt variable.")
	}

	// If the URL does not begin with HTTP, exit.
	if !strings.HasPrefix(checkURL, "http") {
		log.Fatalln("Given URL does not declare a supported protocol. (http | https)")
	}
}

func main() {

	countInt, err := strconv.Atoi(count)
	if err != nil {
		log.Fatalln(err)
	}

	secondInt, err := strconv.Atoi(seconds)
	if err != nil {
		log.Fatalln(err)
	}

	log.Infoln("Beginning check.")
	checksRan := 0
	checksPassed := 0
	checksFailed := 0

	ticker := time.NewTicker(time.Duration(secondInt))
	defer ticker.Stop()
	// This for loop makes a http GET request to a known internet address, address can be changed in deployment spec yaml
	// and returns a http status every second.

	for checksRan < countInt {
		<-ticker.C
		r, err := http.Get(checkURL)
		checksRan++

		if err != nil {
			checksFailed++
			log.Infoln("Failed to reach URL: ", checkURL)
			continue
		}

		if r.StatusCode != http.StatusOK {
			log.Infoln("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
			checksFailed++
			continue
		}
		log.Infoln("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
		checksPassed++
	}

	// Displays the results of 10 URL requests
	log.Infoln(checksRan, "checks ran")
	log.Infoln(checksPassed, "checks passed")
	log.Infoln(checksFailed, "checks failed")

	// Check to see if the 10 requests passed at 80% and reports to Kuberhealthy accordingly
	if checksPassed < 8 {
		reportErr := fmt.Errorf("unable to retrieve a valid response from " + checkURL + "check failed " + strconv.Itoa(checksFailed) + " out of 10 attempts")
		err := kh.ReportFailure([]string{reportErr.Error()})
		if err != nil {
			log.Fatalln("error when reporting to kuberhealthy:", err.Error())
		}
		log.Infoln("Reported Failure to Kuberhealthy")
		os.Exit(0)
	}

	err = kh.ReportSuccess()
	if err != nil {
		log.Fatalln("error when reporting to kuberhealthy:", err.Error())
	}
	log.Infoln("Successfully reported to Kuberhealthy")
}
