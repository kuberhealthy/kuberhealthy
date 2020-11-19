package main

import (
	"errors"
	"strings"
	"time"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

func findValEffect(find string) (string, string, error) {
	if len(find) < 1 {
		return "", "", errors.New("Empty string in findValEffect")
	}
	findte := strings.Split(find, ":")
	if len(findte) > 1 {
		tvalue := findte[0]
		teffect := findte[1]
		if len(tvalue) < 1 {
			return "", "", errors.New("Empty value after split on :")
		}
		return tvalue, teffect, nil
	}
	return find, "", nil
}

func createToleration(toleration string) (*corev1.Toleration, error) {
	t := corev1.Toleration{}
	if len(toleration) < 1 {
		errorMessage := "Must pass toleration value to createToleration"
		return &t, errors.New(errorMessage)
	}
	splitkv := strings.Split(toleration, "=")
	//does toleration has a value provided
	if len(splitkv) > 1 {
		if len(splitkv[0]) < 1 {
			return &t, errors.New("Empty key after split on =")
		}
		tvalue, teffect, err := findValEffect(splitkv[1])
		if err != nil {
			log.Errorln(err)
			return &t, err
		}
		t = corev1.Toleration{
			Key:      splitkv[0],
			Operator: corev1.TolerationOpEqual,
			Value:    tvalue,
			Effect:   corev1.TaintEffect(teffect),
		}
		return &t, nil
	}
	t = corev1.Toleration{
		Key:      toleration,
		Operator: corev1.TolerationOpExists,
	}
	return &t, nil
}

// parseInputValues parses and sets global vars from env variables and other inputs
func parseInputValues() {

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

	// Use injected pod variable KH_CHECK_RUN_DEADLINE to set check timeout
	checkDeadline = now.Add(defaultCheckDeadline)
	var err error
	khDeadline, err = kh.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}

	// Give check just until the shutdownGracePeriod to finish before it hits the kh deadline
	checkDeadline = khDeadline.Add(-shutdownGracePeriod)
	log.Infoln("Check deadline in", checkDeadline.Sub(now))

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

	// Parse incoming check daemonset name
	checkDSName = defaultCheckDSName
	if len(checkDSNameEnv) != 0 {
		checkDSName = checkDSNameEnv
		log.Infoln("Parsed CHECK_DAEMONSET_NAME:", checkDSName)
	}
	log.Infoln("Setting check daemonset name to:", checkDSName)

	// Parse incoming check daemonset name
	podPriorityClassName = defaultPodPriorityClassName
	if len(podPriorityClassNameEnv) != 0 {
		podPriorityClassName = podPriorityClassNameEnv
		log.Infoln("Parsed PRIORITY_CLASS:", podPriorityClassName)
	}
	log.Infoln("Setting check priority class name to:", podPriorityClassName)

	// Parse incoming deployment node selectors
	if len(dsNodeSelectorsEnv) != 0 {
		splitEnvVars := strings.Split(dsNodeSelectorsEnv, ",")
		for _, splitEnvVarKeyValuePair := range splitEnvVars {
			parsedEnvVarKeyValuePair := strings.Split(splitEnvVarKeyValuePair, "=")
			if len(parsedEnvVarKeyValuePair) != 2 {
				log.Warnln("Unable to parse key value pair:", splitEnvVarKeyValuePair)
				continue
			}
			if _, ok := dsNodeSelectors[parsedEnvVarKeyValuePair[0]]; !ok {
				dsNodeSelectors[parsedEnvVarKeyValuePair[0]] = parsedEnvVarKeyValuePair[1]
			}
		}
		log.Infoln("Parsed NODE_SELECTOR:", dsNodeSelectors)
	}
	// Parse incoming deployment tolerations
	if len(tolerationsEnv) != 0 {
		splitEnvVars := strings.Split(tolerationsEnv, ",")
		//do we have multiple tolerations
		if len(splitEnvVars) > 1 {
			for _, toleration := range splitEnvVars {
				//parse each toleration, create a corev1.Toleration object, and append to tolerations slice
				tol, err := createToleration(toleration)
				if err != nil {
					log.Errorln(err)
					continue
				}
				tolerations = append(tolerations, *tol)
			}
		}
		//parse single toleration and append to slice
		tol, err := createToleration(tolerationsEnv)
		if err != nil {
			log.Errorln(err)
			return
		}
		tolerations = append(tolerations, *tol)
		if len(tolerations) > 1 {
			log.Infoln("Parsed TOLERATIONS:", tolerations)
		}
	}
}
