package main

import (
	"strings"
	"time"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

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
				splitkv := strings.Split(toleration, "=")
				//does toleration has a value provided
				if len(splitkv) > 1 {
					findte := strings.Split(splitkv[1], ":")
					//does toleration have an effect
					if len(findte) > 1 {
						//get value/effect and generate toleration
						tvalue := findte[0]
						teffect := findte[1]
						t := corev1.Toleration{
							Key: splitkv[0],
							Operator: corev1.TolerationOpEqual,
							Value: tvalue,
							Effect: corev1.TaintEffect(teffect),
						}
						tolerations = append(tolerations, t)
					} else {
						// generate based on splitkv
						t := corev1.Toleration{
							Key: splitkv[0],
							Operator: corev1.TolerationOpEqual,
							Value: splitkv[1],
						}
						tolerations = append(tolerations, t)
					}
				} else {
					t := corev1.Toleration{
						Key: toleration,
						Operator: corev1.TolerationOpExists,
					}
					tolerations = append(tolerations, t)
				}
			}
		} else {
			//generate toleration based on single string value
			t := corev1.Toleration{
				Key: tolerationsEnv,
				Operator: corev1.TolerationOpExists,
		        }
			tolerations = append(tolerations, t)
		}
		log.Infoln("Parsed TOLERATIONS:", tolerations)
	} else {
		log.Infoln("Unable to parse tolerations without TOLERATIONS value set.")
	}
}
