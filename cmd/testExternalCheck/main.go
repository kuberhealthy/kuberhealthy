package main

import (
	"log"
	"os"
	"strconv"
	"time"

	checkclient "github.com/Comcast/kuberhealthy/pkg/checks/external/checkClient"
)

var reportFailure bool

func init() {
	var err error
	reportFailureStr := os.Getenv("REPORT_FAILURE")
	reportFailure, err = strconv.ParseBool(reportFailureStr)
	if err != nil {
		log.Fatalln("Failed to parse REPORT_FAILURE env var:", err)
	}
}

func main() {
	log.Println("Waiting 10 seconds before reporting success...")
	time.Sleep(time.Second * 10)

	var err error
	if reportFailure {
		err = checkclient.ReportFailure([]string{"Test has failed!"})
	} else {
		err = checkclient.ReportSuccess()
	}

	if err != nil {
		log.Println("Error reporting to Kuberhealthy servers:", err)
		return
	}
	log.Println("Successfully reported to Kuberhealthy servers")
}
