package main

import (
	"strings"
	"time"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
)

// parseInputValues parses and sets global vars from env variables and other inputs
func parseInputValues() {

	// Use injected pod variable KH_CHECK_RUN_DEADLINE to set check time limit
	checkTimeLimit = defaultCheckTimeLimit
	timeDeadline, err := kh.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}
	// Give 60 seconds to clean up / shut down once check time limit is reached before the kh deadline
	checkTimeLimit = timeDeadline.Sub(time.Now().Add(time.Second * 60))
	log.Infoln("Setting check time limit to:", checkTimeLimit)

	// Parse incoming namespace environment variable
	checkNamespace = defaultCheckNamespace
	if len(checkNamespaceEnv) != 0 {
		checkNamespace = checkNamespaceEnv
		log.Infoln("Parsed POD_NAMESPACE:", checkNamespace)
	}
	log.Infoln("Performing check in", checkNamespace, "namespace.")

	// Allow user to override the image used by the daemonset check - see #114
	dsPauseContainerImage = defaultDSPauseContainerImage
	if len(dsPauseContainerImageEnv) > 0 {
		log.Infoln("Parsed PAUSE_CONTAINER_IMAGE:", dsPauseContainerImageEnv)
		dsPauseContainerImage = dsPauseContainerImageEnv
	}
	log.Infoln("Setting DS pause container image to:", dsPauseContainerImage)

	// Parse incoming deployment node selectors
	if len(dsNodeSelectorsEnv) != 0 {
		splitEnvVars := strings.Split(dsNodeSelectorsEnv, ",")
		for _, splitEnvVarKeyValuePair := range splitEnvVars {
			parsedEnvVarKeyValuePair := strings.Split(splitEnvVarKeyValuePair, "=")
			if _, ok := dsNodeSelectors[parsedEnvVarKeyValuePair[0]]; !ok {
				dsNodeSelectors[parsedEnvVarKeyValuePair[0]] = parsedEnvVarKeyValuePair[1]
			}
		}
		log.Infoln("Parsed NODE_SELECTOR:", dsNodeSelectors)
	}

	// Parse incoming custom shutdown grace period seconds
	shutdownGracePeriod = defaultShutdownGracePeriod
	if len(shutdownGracePeriodEnv) != 0 {
		duration, err := time.ParseDuration(shutdownGracePeriodEnv)
		if err != nil {
			log.Fatalln("error occurred attempting to parse SHUTDOWN_GRACE_PERIOD:", err)
		}
		if duration.Minutes() < 1 {
			log.Fatalln("error occurred attempting to parse SHUTDOWN_GRACE_PERIOD. A value less than 1 was parsed:", duration.Minutes())
		}
		shutdownGracePeriod = duration
		log.Infoln("Parsed SHUTDOWN_GRACE_PERIOD:", shutdownGracePeriod)
	}
	log.Infoln("Setting shutdown grace period to:", shutdownGracePeriod)

	// Parse incoming check daemonset name
	checkDSName = defaultCheckDSName
	if len(checkDSNameEnv) != 0 {
		checkDSName = checkDSNameEnv
		log.Infoln("Parsed CHECK_DAEMONSET_NAME:", checkDSName)
	}
	log.Infoln("Setting check daemonset name to:", checkDSName)

}
