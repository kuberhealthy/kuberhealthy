package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
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
	log.Println("Beginning check.")
	checksRan := 0
	checksPassed := 0
	checksFailed := 0

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	// This for loop makes a http GET request to a known internet address, address can be changed in deployment spec yaml
	// and returns a http status every second.
	for checksRan < 10 {
		<-ticker.C
		r, err := http.Get(checkURL)
		checksRan++

		if err != nil {
			checksFailed++
			log.Println("Failed to reach URL: ", checkURL)
			continue
		}

		if r.StatusCode != http.StatusOK {
			log.Println("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
			checksFailed++
			continue
		}
		log.Println("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
		checksPassed++
	}

	// Displays the results of 10 URL requests
	log.Println(checksRan, "checks ran")
	log.Println(checksPassed, "checks passed")
	log.Println(checksFailed, "checks failed")

	// Check to see if the 10 requests passed at 80% and reports to Kuberhealthy accordingly
	if checksPassed < 8 {
		reportErr := fmt.Errorf("unable to retrieve a http.StatusOK from " + checkURL + "check failed " + strconv.Itoa(checksFailed) + " out of 10 attempts")
		err := kh.ReportFailure([]string{reportErr.Error()})
		if err != nil {
			log.Fatalln("error when reporting to kuberhealthy:", err.Error())
		}
		log.Println("Successfully reported to Kuberhealthy of failure")
	}

	err := kh.ReportSuccess()
	if err != nil {
		log.Fatalln("error when reporting to kuberhealthy:", err.Error())
	}
	log.Println("Successfully reported to Kuberhealthy")
}
