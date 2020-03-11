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
	// Parse whitelist and blacklist enablation.
	if len(whitelistOnEnv) != 0 {
		whitelistEnabled, err := strconv.ParseBool(whitelistOnEnv)
		if err != nil {
			log.Fatalln("error occured attempting to parse WHITELIST_ON:", err)
		}
		log.Infoln("Parsed WHITELIST_ON:", whitelistEnabled)
		whitelistOn = whitelistEnabled
	} else if len(blacklistOnEnv) != 0 {
		blacklistEnabled, err := strconv.ParseBool(blacklistOnEnv)
		if err != nil {
			log.Fatalln("error occured attempting to parse BLACKLIST_ON:", err)
		}
		log.Infoln("Parsed BLACKLIST_ON:", blacklistEnabled)
		blacklistOn = blacklistEnabled
	}

	if whitelistOn && blacklistOn {
		log.Infoln("Whitelist and blacklist are both enabled, looking at whitelisted namespaces except specified blacklisted namespaces.")
	}

	// Parse blacklist and whitelist namespaces.
	if len(blacklistNamespacesEnv) != 0 {
		blacklistNamespaces = strings.Split(blacklistNamespacesEnv, ",")
		log.Infoln("Parsed BLACKLIST_NAMESPACES:", blacklistNamespaces)
	}
	if len(whitelistNamespacesEnv) != 0 {
		whitelistNamespaces = strings.Split(whitelistNamespacesEnv, ",")
		log.Infoln("Parsed WHITELIST_NAMESPACES:", whitelistNamespaces)
	}

	switch {
	case whitelistOn && blacklistOn:
		log.Infoln("Looking at", whitelistNamespaces, "but ignoring", blacklistNamespaces)
	case whitelistOn:
		log.Infoln("Looking at", whitelistNamespaces)
	case blacklistOn:
		log.Infoln("Ignoring", blacklistNamespaces)
	default:
		log.Infoln("Looking at all namespaces.")
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
		log.Infoln("Parsed CHECK_TIME_LIMIT:", duration.Seconds())
		checkTimeLimit = duration
	}
	log.Infoln("Check time limit set to:", checkTimeLimit)
}
