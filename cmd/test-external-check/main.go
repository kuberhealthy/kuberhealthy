package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external"
	checkclient "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
)

var reportFailure bool
var reportDelay time.Duration

var (
	timeLimit time.Duration
)

func init() {

	// enable debug logging on the check client
	checkclient.Debug = true

	var err error

	// parse REPORT_FAILURE environment var
	reportFailureStr := os.Getenv("REPORT_FAILURE")
	if len(reportFailureStr) != 0 {
		reportFailure, err = strconv.ParseBool(reportFailureStr)
		if err != nil {
			log.Fatalln("Failed to parse REPORT_FAILURE env var:", err)
		}
	}

	// parse REPORT_DELAY environment var
	reportDelayStr := os.Getenv("REPORT_DELAY")
	reportDelay = time.Duration(time.Second * 5)
	if len(reportDelayStr) != 0 {
		reportDelay, err = time.ParseDuration(reportDelayStr)
		if err != nil {
			log.Fatalln("Failed to parse REPORT_DELAY env var:", err)
		}
	}

	// Set check time limit to default
	timeLimit = time.Duration(time.Minute * 10)
	// Get the deadline time in unix from the env var
	timeDeadline, err := checkclient.GetDeadline()
	if err != nil {
		log.Println("There was an issue getting the check deadline:", err.Error())
	}
	timeLimit = timeDeadline.Sub(time.Now().Add(time.Second * 5))
	log.Println("Check time limit set to:", timeLimit)
}

func main() {

	log.Println("Using kuberhealthy reporting url", os.Getenv(external.KHReportingURL))
	log.Println("Waiting", reportDelay, "seconds before reporting...")
	time.Sleep(reportDelay)

	go func() {
		select {
		case <-time.After(timeLimit):
			log.Println("Check took too long and timed out.")
			os.Exit(1)
		}
	}()

	var err error
	if reportFailure {
		log.Println("Reporting failure...")
		err = checkclient.ReportFailure([]string{"Test has failed!"})
		if err != nil {
			log.Println(err.Error())
		}
	} else {
		log.Println("Reporting success...")
		err = checkclient.ReportSuccess()
		if err != nil {
			log.Println(err.Error())
		}
	}

	if err != nil {
		log.Println("Error reporting to Kuberhealthy servers:", err)
		return
	}
	log.Println("Successfully reported to Kuberhealthy servers")
}
