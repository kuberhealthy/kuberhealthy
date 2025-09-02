package main

import (
	"log"
	"os"

	checkclient "github.com/kuberhealthy/kuberhealthy/v3/pkg/checkclient"
)

// main demonstrates how an external check can report its status back to
// Kuberhealthy. The checker uses environment variables provided to the pod
// by Kuberhealthy to know where and how to report results.
func main() {
	reportingURL := os.Getenv("KH_REPORTING_URL")
	runUUID := os.Getenv("KH_RUN_UUID")
	log.Printf("reporting to %s with run UUID %s", reportingURL, runUUID)

	// Set FAIL=true to demonstrate a failing result.
	if os.Getenv("FAIL") == "true" {
		err := checkclient.ReportFailure([]string{"example failure"})
		if err != nil {
			log.Fatalf("failed to report failure: %v", err)
		}
		return
	}

	err := checkclient.ReportSuccess()
	if err != nil {
		log.Fatalf("failed to report success: %v", err)
	}
}
