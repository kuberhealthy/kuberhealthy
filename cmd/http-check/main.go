package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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

	// Make a GET request.
	for checksRan < 9 {
		r, err := http.Get(checkURL)
		checksRan = checksRan + 1

		if r.StatusCode == http.StatusOK {
			log.Println("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
			kh.ReportSuccess()
			os.Exit(0)
		}

		if err != nil {
			checksFailed = checksFailed + 1
			// log.Println("Got a", r.StatusCode, "with a", http.MethodGet, "to", checkURL)
			log.Println("Failed to reach URL: ", checkURL, " recieved a ", r.StatusCode)
			continue
		}
		checksPassed = checksPassed + 1
		log.Println("Debug Test")

	}

	log.Println(checksRan)
	log.Println(checksPassed)
	log.Println(checksFailed)

	// Check to see if the 10 requests passed at 80%
	if checksPassed >= 8 {
		err := kh.ReportSuccess()
		if err != nil {
			log.Println("error when reporting to kuberhealthy:", err.Error())
			log.Fatalln("error when reporting to kuberhealthy:", err.Error())
			os.Exit(0)
		} else {
			reportErr := fmt.Errorf("unable to retrieve a response from " + checkURL)
			log.Println("Error retrieving URL: ", checksFailed, " out of 10 attempts")
			err := kh.ReportFailure([]string{reportErr.Error()})
			if err != nil {
				log.Fatalln("error when reporting to kuberhealthy:", err.Error())
			}
			os.Exit(0)
		}
	}
}
