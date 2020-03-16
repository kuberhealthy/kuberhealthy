package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// parseDebugSettings parses debug settings and fatals on errors.
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

// parseInputValues parses all incoming environment variables for the program into globals and fatals on errors.
func parseInputValues() {

	// Parse blacklist and whitelist namespaces.
	if len(blacklistEnv) != 0 {
		blacklist = strings.Split(blacklistEnv, ",")
		log.Infoln("Parsed BLACKLIST:", blacklist)
	}

	if len(whitelistEnv) != 0 {
		whitelist = strings.Split(whitelistEnv, ",")
		log.Infoln("Parsed WHITELIST:", whitelist)
	}

	// Parse memory and CPU thresholds.
	// (0.90 represents 90% and will alert if usage is at least 90% inclusive)
	if len(thresholdEnv) != 0 {
		var err error
		threshold, err = strconv.ParseFloat(thresholdEnv, 64)
		if err != nil {
			log.Fatalln("error occurred attempting to parse THRESHOLD:", err)
		}
		log.Infoln("Parsed THRESHOLD:", threshold)
	}
	if threshold > 0.99 {
		log.Infoln("Given THRESHOLD is greater than 0.99, setting to default of", defaultThreshold)
		threshold = defaultThreshold
	}
	if threshold <= 0 {
		log.Infoln("Threshold is less than or equal to 0, setting to default of", defaultThreshold)
		threshold = defaultThreshold
	}
	log.Infoln("Usage threshold set to:", threshold)

	// Set check time limit to default.
	checkTimeLimit = defaultCheckTimeLimit
	if len(checkTimeLimitEnv) != 0 {
		duration, err := time.ParseDuration(checkTimeLimitEnv)
		if err != nil {
			log.Fatalln("error occurred attempting to parse CHECK_TIME_LIMIT:", err)
		}
		if duration.Seconds() < 1 {
			log.Fatalln("error occurred attempting to parse CHECK_TIME_LIMIT. Check run time in seconds is less than 1:", duration.Seconds())
		}
		log.Infof("Parsed CHECK_TIME_LIMIT: %.0f seconds", duration.Seconds())
		checkTimeLimit = duration
	}
	log.Infof("Check time limit set to: %.0f seconds", checkTimeLimit.Seconds())
}
