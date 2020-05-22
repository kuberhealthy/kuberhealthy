package main

import (
	"time"

	log "github.com/sirupsen/logrus"
	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"

)

// parseInputValues parses and sets global vars from env variables and other inputs
func parseInputValues() {

	// Use injected pod variable KH_CHECK_RUN_DEADLINE to set check time limit
	checkTimeLimit = defaultCheckTimeLimit
	timeDeadline, err := kh.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}
	checkTimeLimit = timeDeadline.Sub(time.Now().Add(time.Second * 5))
	log.Infoln("Setting check time limit to:", checkTimeLimit)

	// Parse incoming namespace environment variable
	checkNamespace = defaultCheckNamespace
	if len(checkNamespaceEnv) != 0 {
		checkNamespace = checkNamespaceEnv
		log.Infoln("Parsed POD_NAMESPACE:", checkNamespace)
	}
	log.Infoln("Performing check in", checkNamespace, "namespace.")

	// Parse incoming check pod timeout environment variable
	checkPodTimeout = defaultCheckPodTimeout
	if len(checkPodTimeoutEnv) != 0 {
		duration, err := time.ParseDuration(checkPodTimeoutEnv)
		if err != nil {
			log.Fatalln("error parsing env variable CHECK_POD_TIMEOUT:", checkPodTimeoutEnv, err)
		}
		if duration.Minutes() < 1 {
			log.Fatalln("error parsing env variable CHECK_POD_TIMEOUT. A value of less than 1 was parsed:", duration.Minutes())
		}
		if duration.Minutes() > checkTimeLimit.Minutes() {
			log.Fatalln("error parsing env variable CHECK_POD_TIMEOUT. Value is greater than checkTimeLimit:", checkTimeLimit.Minutes())
		}
		checkPodTimeout = duration
		log.Infoln("Parsed CHECK_POD_TIMEOUT:", checkPodTimeout)
	}
	log.Infoln("Setting check pod timeout to:", checkPodTimeout)

	// Allow user to override the image used by the daemonset check - see #114
	dsPauseContainerImage = defaultDSPauseContainerImage
	if len(dsPauseContainerImageEnv) > 0 {
		log.Infoln("Parsed PAUSE_CONTAINER_IMAGE:", dsPauseContainerImageEnv)
		dsPauseContainerImage = dsPauseContainerImageEnv
	}
	log.Infoln("Setting DS pause container image to:", dsPauseContainerImage)

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

