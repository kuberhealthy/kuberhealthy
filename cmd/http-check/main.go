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
	passing  = os.Getenv("PASSING_PERCENT")
)

func init() {
	// Check that the URL environment variable is valid.
	if len(checkURL) == 0 {
		log.Errorln("Empty URL. Please update your CHECK_URL environment variable.")
		kh.ReportFailure([]string{"CHECK_URL environment variable not set."})
		os.Exit(0)
	}

	// Check that the COUNT environment variable is valid.
	if len(count) == 0 {
		log.Errorln("Empty count value. Please update your COUNT environemnt variable.")
		kh.ReportFailure([]string{"COUNT environment variable not set."})
		os.Exit(0)
	}

	// Check that the SECONDS environment variable is valid.
	if len(seconds) == 0 {
		log.Errorln("Empty seconds value. Please update your SECONDS environemnt variable.")
		kh.ReportFailure([]string{"SECONDS environment variable not set."})
		os.Exit(0)
	}

	// If the URL does not begin with HTTP, exit.
	if !strings.HasPrefix(checkURL, "http") {
		log.Errorln("Given URL does not declare a supported protocol. (http | https)")
		kh.ReportFailure([]string{"Given URL does not declare a supported protocol. (http | https)"})
		os.Exit(0)
	}
}

func main() {

	countInt64, err := strconv.ParseInt(count, 10, 0)
	if err != nil {
		log.Fatalln(err)
	}

	secondInt, err := strconv.Atoi(seconds)
	if err != nil {
		log.Fatalln(err)
	}

	passingInt, err := strconv.ParseInt(passing, 10, 0)
	if err != nil {
		log.Fatalln(err)
	}

	passingPercentage := passingInt / 100

	countInt := int(countInt64)

	// sets the passing score to compare it against checks that have been ran
	passingScore := passingPercentage * countInt64
	passInt := int(passingScore)
	log.Infoln("Looking for at least", passing, "percent of", count, "checks to pass")

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
			log.Errorln("Failed to reach URL: ", checkURL)
			continue
		}

		if r.StatusCode != http.StatusOK {
			log.Errorln("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
			checksFailed++
			continue
		}
		log.Errorln("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
		checksPassed++
	}

	// Displays the results of 10 URL requests
	log.Infoln(checksRan, "checks ran")
	log.Infoln(checksPassed, "checks passed")
	log.Infoln(checksFailed, "checks failed")

	// Check to see if the number of requests passed at passingPercent and reports to Kuberhealthy accordingly
	if checksPassed < passInt {
		reportErr := fmt.Errorf("unable to retrieve a valid response from " + checkURL + "check failed " + strconv.Itoa(checksFailed) + " out of " + strconv.Itoa(checksRan) + " attempts")
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
