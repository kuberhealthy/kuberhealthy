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
	if len(blacklistOnEnv) != 0 {
		blacklistEnabled, err := strconv.ParseBool(blacklistOnEnv)
		if err != nil {
			log.Fatalln("error occured attempting to parse BLACKLIST_ON:", err)
		}
		log.Infoln("Parsed BLACKLIST_ON:", blacklistEnabled)
		blacklistOn = blacklistEnabled
		whitelistOn = !blacklistEnabled
	} else if len(whitelistOnEnv) != 0 {
		whitelistEnabled, err := strconv.ParseBool(whitelistOnEnv)
		if err != nil {
			log.Fatalln("error occured attempting to parse WHITELIST_ON:", err)
		}
		log.Infoln("Parsed WHITELIST_ON:", whitelistEnabled)
		whitelistOn = whitelistEnabled
		blacklistOn = !whitelistEnabled
	}

	if len(namespacesEnv) != 0 {
		namespaces = strings.Split(namespacesEnv, ",")
		log.Infoln("Parsed NAMESPACES:", namespaces)
		switch {
		case whitelistOn:
			log.Infoln("Looking at", namespaces)
		default:
			log.Infoln("Ignoring", namespaces)
		}
	}

	// Set check time limit to default
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
