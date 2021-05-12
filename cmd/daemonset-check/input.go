package main

import (
	"errors"
	"strings"
	"time"

	kh "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

func findValEffect(find string) (string, string, error) {
	//Is the find string not null?
	if len(find) < 1 {
		return "", "", errors.New("Empty string in findValEffect")
	}
	findTe := strings.Split(find, ":")
	if len(findTe) > 1 {
		//if we split the string, and the value isn't null (we don't have ":something"), return the values
		tValue := findTe[0]
		tEffect := findTe[1]
		if len(tValue) < 1 {
			return "", "", errors.New("Empty value after split on :")
		}
		return tValue, tEffect, nil
	}
	// if we couldn't split the string, just return it back as the value
	return find, "", nil
}

func createToleration(toleration string) (*corev1.Toleration, error) {
	t := corev1.Toleration{}

	// Ensure we were supplied a toleration string
	if len(toleration) < 1 {
		errorMessage := "Must pass toleration value to createToleration"
		return &t, errors.New(errorMessage)
	}
	splitKV := strings.Split(toleration, "=")
	//does toleration has a value provided
	if len(splitKV) > 1 {
		//make sure there's real values on both sides of the split
		if len(splitKV[0]) < 1 {
			return &t, errors.New("Empty key after split on =")
		}
		// try to get value and effect by splitting on :
		tValue, tEffect, err := findValEffect(splitKV[1])
		if err != nil {
			log.Errorln(err)
			return &t, err
		}
		// create toleration based on returned value/effect
		t = corev1.Toleration{
			Key:      splitKV[0],
			Operator: corev1.TolerationOpEqual,
			Value:    tValue,
			Effect:   corev1.TaintEffect(tEffect),
		}
		return &t, nil
	}
	// if no split can be done, just create a toleration based on the supplied string
	t = corev1.Toleration{
		Key:      toleration,
		Operator: corev1.TolerationOpExists,
	}
	return &t, nil
}

func createTaintMap(taintMap map[string]corev1.TaintEffect, taint string) error {

	splitKV := strings.Split(taint, "=")
	// does toleration has a value provided
	if len(splitKV) > 1 {
		//make sure there's real values on both sides of the split
		if len(splitKV[0]) < 1 {
			return errors.New("Empty key after split on =")
		}

		// try to get effect by splitting on :
		_, tEffect, err := findValEffect(splitKV[1])
		if err != nil {
			log.Errorln(err)
			return err
		}

		tKey := splitKV[0]
		taintMap[tKey] = corev1.TaintEffect(tEffect)
		return nil
	}

	// Taint sometimes only has KEY and EFFECT but no VALUE provided -- which is normally the case for cordoned nodes
	// eg. node.kubernetes.io/unschedulable:NoSchedule
	splitKE := strings.Split(taint, ":")

	// make sure Taint has key and effect
	if len(splitKE[0]) < 1 {
		return errors.New("Empty key after split on :")
	}

	tKey := splitKE[0]
	tEffect := splitKE[1]
	taintMap[tKey] = corev1.TaintEffect(tEffect)
	return nil
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
					// if we can't get a toleration based on that string, skip it and go on to the next one
					log.Errorln(err)
					continue
				}
				tolerations = append(tolerations, *tol)
			}
		}
		//parse single toleration and append to slice
		tol, err := createToleration(tolerationsEnv)
		if err != nil {
			// if we can't create a toleration, error out and return
			log.Errorln(err)
			return
		}
		tolerations = append(tolerations, *tol)
		// if we parsed tolerations, log them
		if len(tolerations) > 1 {
			log.Infoln("Parsed TOLERATIONS:", tolerations)
		}
	}

	if len(allowedTaintsEnv) != 0 {
		allowedTaints = make(map[string]corev1.TaintEffect)
		splitEnvVars := strings.Split(allowedTaintsEnv, ",")
		for _, taint := range splitEnvVars {

			err = createTaintMap(allowedTaints, taint)
			if err != nil {
				// if we can't get a taint based on that string, skip it and go on to the next one
				log.Errorln(err)
				continue
			}
		}

		if len(allowedTaints) != 0 {
			log.Infoln("Parsed ALLOWED_TAINTS:", allowedTaints)
		}
	}
}
