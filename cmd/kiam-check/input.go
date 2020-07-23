package main

import (
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
)

// parseDebugSettings parses incoming debug settings (environment variables).
func parseDebugSettings() {

	// Enable debug logging if required.
	if len(debugEnv) != 0 {
		var err error
		debug, err = strconv.ParseBool(debugEnv)
		if err != nil {
			log.Fatalln("failed to parse DEBUG environment variable:", err)
		}
	}

	// Turn on debug logging.
	if debug {
		log.Infoln("Debug logging enabled.")
		log.SetLevel(log.DebugLevel)
	}
	log.Debugln(os.Args)
}

// parseInputValues parses incoming input values (environment variables).
func parseInputValues() {

	// Parse incoming AWS region.
	awsRegion = defaultAWSRegion
	if len(awsRegionEnv) != 0 {
		awsRegion = awsRegionEnv
		log.Infoln("Parsed AWS_REGION:", awsRegion)
	}

	// Parse incoming expected Lambda Count.
	if len(expectedLambdaCountEnv) != 0 {
		count, err := strconv.Atoi(expectedLambdaCountEnv)
		if err != nil {
			log.Fatalln("error occurred attempting to parse LAMBDA_COUNT:", err)
		}
		expectedLambdaCount = count
		log.Infoln("Parsed LAMBDA_COUNT:", expectedLambdaCount)
	}
}
