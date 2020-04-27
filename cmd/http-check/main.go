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
		os.Exit(0)
	}

	// If the URL does not begin with HTTP, exit.
	if !strings.HasPrefix(checkURL, "http") {
		log.Fatalln("Given URL does not declare a supported protocol. (http | https)")
		os.Exit(0)
	}
}

func main() {
	log.Println("Beginning check.")
	checksRan := 0
	checksPassed := 0
	checksFailed := 0

	// Make a GET request.
	for checksRan < 10 {
		r, err := http.Get(checkURL)
		checksRan = checksRan + 1

		if err != nil {
			checksFailed = checksFailed + 1
			// log.Println("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
			// log.Println("Failed to reach URL: ", checkURL, " recieved a ", r.StatusCode)
			log.Println("Failed to reach URL: ", checkURL)
			continue
		}

		if r.StatusCode == http.StatusOK {
			log.Println("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
			checksPassed = checksPassed + 1
			continue
		}

		if r.StatusCode != http.StatusOK {
			log.Println("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
			checksFailed = checksFailed + 1
			continue
		}
		time.Sleep(time.Second)
	}

	// Debug logging counts
	log.Println(checksRan, "checks ran")
	log.Println(checksPassed, "checks passed")
	log.Println(checksFailed, "checks failed")

	// Check to see if the 10 requests passed at 80%
	if checksPassed >= 8 {
		err := kh.ReportSuccess()
		if err != nil {
			log.Fatalln("error when reporting to kuberhealthy:", err.Error())
			os.Exit(0)
		}
		log.Println("Successfully reported to Kuberhealthy")
	}

	// Kuberhealthy reporting when checks passed is less than 8 out of 10 attempts
	if checksPassed < 8 {
		reportErr := fmt.Errorf("unable to retrieve a http.StatusOK from " + checkURL + "check failed " + strconv.Itoa(checksFailed) + " out of 10 attempts")
		// log.Println(reportErr.Error())
		err := kh.ReportFailure([]string{reportErr.Error()})
		if err != nil {
			log.Fatalln("error when reporting to kuberhealthy:", err.Error())
			os.Exit(0)
		}
		log.Println("Successfully reported to Kuberhealthy of failure")
	}
}
