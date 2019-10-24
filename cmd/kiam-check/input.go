package main

import (
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
)

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

func parseInputValues() {

	var err error

	// Parse incoming AWS region.
	awsRegion = defaultAWSRegion
	if len(awsRegionEnv) != 0 {
		awsRegion = awsRegionEnv
		log.Infoln("Parsed AWS_REGION:", awsRegion)
	}

	// Respect the request for using of AWS Lambdas.
	useLambdas, err = strconv.ParseBool(useLambdasEnv)
	if err != nil {
		log.Fatalln("failed to parse LAMBDAS:", err)
	}

	if useLambdas {
		// Parse incoming expected Lambda Count.
		if len(expectedLambdaCountEnv) != 0 {
			count, err := strconv.Atoi(expectedLambdaCountEnv)
			if err != nil {
				log.Fatalln("error occured attempting to parse LAMBDA_COUNT:", err)
			}
			expectedLambdaCount = count
			log.Infoln("Parsed LAMBDA_COUNT:", expectedLambdaCount)
		}
	}
}
