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
	checksRan := 0
	checksPassed := 0
	checksFailed := 0

	// Make a GET request.
	for checksRan <= 10 {
		R, err := http.Get(checkURL)
		checksRan = checksRan + 1
		if err != nil && R.StatusCode != http.StatusOK {
			checksFailed = checksFailed + 1
			reportErr := fmt.Errorf("error occurred performing a " + http.MethodGet + " to " + checkURL + ": " + err.Error())
			log.Errorln(reportErr.Error())
			log.Infoln("Got a", R.StatusCode, "with a", http.MethodGet, "to", checkURL)
			continue
		}
		checksPassed = checksPassed + 1

		// Check if the response status code is a 200.
		if checksPassed >= 8 {
			err = kh.ReportSuccess()
			if err != nil {
				log.Fatalln("error when reporting to kuberhealthy:", err.Error())
				os.Exit(0)
				continue
			}
			reportErr := fmt.Errorf("unable to retrieve a " + strconv.Itoa(http.StatusOK) + " from " + checkURL + " got a " + strconv.Itoa(R.StatusCode) + " instead")
			log.Println("Error retrieving URL: ", checksFailed, " out of 10 attempts")
			err = kh.ReportFailure([]string{reportErr.Error()})
			if err != nil {
				log.Fatalln("error when reporting to kuberhealthy:", err.Error())
			}
			os.Exit(0)
		}
	}
}
