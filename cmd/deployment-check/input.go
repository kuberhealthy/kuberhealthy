package main

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	kh "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// parseDebugSettings parses debug settings and fatals on errors.
func parseDebugSettings() {

	// Enable debug logging if required.
	if len(debugEnv) != 0 {
		var err error
		debug, err = strconv.ParseBool(debugEnv)
		if err != nil {
			log.Fatalln("failed to parse DEBUG environment variable:", err.Error())
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

	// Parse incoming check image environment variable.
	checkImageURL = defaultCheckImageURL
	if len(checkImageURLEnv) != 0 {
		checkImageURL = checkImageURLEnv
		log.Infoln("Parsed CHECK_IMAGE:", checkImageURL)
	}

	// Parse incoming check image B environment variable.
	checkImageURLB = defaultCheckImageURLB
	if len(checkImageURLBEnv) != 0 {
		checkImageURLB = checkImageURLBEnv
		log.Infoln("Parsed CHECK_IMAGE_ROLL_TO:", checkImageURLB)
	}

	// Parse image pull secret for check image
	if len(checkImagePullSecretEnv) != 0 {
		checkImagePullSecret = checkImagePullSecretEnv
		log.Infoln("Parsed CHECK_IMAGE_PULL_SECRET:", checkImagePullSecret)
	}

	// Parse incoming check deployment name.
	checkDeploymentName = defaultCheckDeploymentName
	if len(checkDeploymentNameEnv) != 0 {
		checkDeploymentName = checkDeploymentNameEnv
		log.Infoln("Parsed CHECK_DEPLOYMENT_NAME:", checkDeploymentName)
	}

	// Parse incoming check service name.
	checkServiceName = defaultCheckServiceName
	if len(checkServiceNameEnv) != 0 {
		checkServiceName = checkServiceNameEnv
		log.Infoln("Parsed CHECK_SERVICE_NAME:", checkServiceName)
	}

	// Parse incoming container port environment variable
	checkContainerPort = defaultCheckContainerPort
	if len(checkContainerPortEnv) != 0 {
		port, err := strconv.Atoi(checkContainerPortEnv)
		if err != nil {
			log.Fatalln("error occurred attempting to parse CHECK_CONTAINER_PORT:", err)
		}
		checkContainerPort = int32(port)
		log.Infoln("Parsed CHECK_CONTAINER_PORT:", checkContainerPort)
	}

	// Parse incoming load balancer port environment variable
	checkLoadBalancerPort = defaultCheckLoadBalancerPort
	if len(checkLoadBalancerPortEnv) != 0 {
		port, err := strconv.Atoi(checkLoadBalancerPortEnv)
		if err != nil {
			log.Fatalln("error occurred attempting to parse CHECK_LOAD_BALANCER_PORT:", err)
		}
		checkLoadBalancerPort = int32(port)
		log.Infoln("Parsed CHECK_LOAD_BALANCER_PORT:", checkLoadBalancerPort)
	}

	// Parse incoming namespace environment variable
	checkNamespace = defaultCheckNamespace
	data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		log.Warnln("Failed to open namespace file:", err.Error())
	}
	if len(data) != 0 {
		log.Infoln("Found pod namespace:", string(data))
		checkNamespace = string(data)
	}
	if len(checkNamespaceEnv) != 0 {
		checkNamespace = checkNamespaceEnv
		log.Infoln("Parsed CHECK_NAMESPACE:", checkNamespace)
	}
	log.Infoln("Performing check in", checkNamespace, "namespace.")

	// Parse incoming deployment replica environment variable
	checkDeploymentReplicas = defaultCheckDeploymentReplicas
	if len(checkDeploymentReplicasEnv) != 0 {
		reps, err := strconv.Atoi(checkDeploymentReplicasEnv)
		if err != nil {
			log.Fatalln("error occurred attempting to parse CHECK_DEPLOYMENT_REPLICAS:", err)
		}
		if reps < 1 {
			log.Fatalln("error occurred attempting to parse CHECK_DEPLOYMENT_REPLICAS.  Replica(s) is less than 1:", reps)
		}
		checkDeploymentReplicas = reps
		log.Infoln("Parsed CHECK_DEPLOYMENT_REPLICAS:", checkDeploymentReplicas)
	}

	// Parse incpoming deployment tolerations
	if len(checkDeploymentTolerationsEnv) > 0 {
		splitEnvVars := strings.Split(checkDeploymentTolerationsEnv, ",")
		for _, splitEnvVarKeyValuePair := range splitEnvVars {
			parsedEnvVarKeyValuePair := strings.Split(splitEnvVarKeyValuePair, "=")
			if len(parsedEnvVarKeyValuePair) != 2 {
				log.Warnln("Unable to parse key value pair:", splitEnvVarKeyValuePair)
				log.Warnln("Setting operator to", corev1.TolerationOpExists)
				t := corev1.Toleration{
					Key:      parsedEnvVarKeyValuePair[0],
					Operator: corev1.TolerationOpExists,
				}
				log.Infoln("Adding toleration to deployment:", t)
				checkDeploymentTolerations = append(checkDeploymentTolerations, t)
				continue
			}
			parsedEnvVarValueEffect := strings.Split(parsedEnvVarKeyValuePair[1], ":")
			if len(parsedEnvVarValueEffect) != 2 {
				log.Warnln("Unable to parse complete toleration value and effect:", parsedEnvVarValueEffect)
				t := corev1.Toleration{
					Key:      parsedEnvVarKeyValuePair[0],
					Operator: corev1.TolerationOpEqual,
					Value:    parsedEnvVarKeyValuePair[1],
				}
				log.Infoln("Adding toleration to deployment:", t)
				checkDeploymentTolerations = append(checkDeploymentTolerations, t)
				continue
			}
			t := corev1.Toleration{
				Key:      parsedEnvVarKeyValuePair[0],
				Operator: corev1.TolerationOpEqual,
				Value:    parsedEnvVarValueEffect[0],
				Effect:   corev1.TaintEffect(parsedEnvVarValueEffect[1]),
			}
			log.Infoln("Adding toleration to deployment:", t)
			checkDeploymentTolerations = append(checkDeploymentTolerations, t)
		}
		log.Infoln("Parsed TOLERATIONS:", checkDeploymentTolerations)
	}

	// Parse incoming deployment node selectors
	if len(checkDeploymentNodeSelectorsEnv) > 0 {
		splitEnvVars := strings.Split(checkDeploymentNodeSelectorsEnv, ",")
		for _, splitEnvVarKeyValuePair := range splitEnvVars {
			parsedEnvVarKeyValuePair := strings.Split(splitEnvVarKeyValuePair, "=")
			if len(parsedEnvVarKeyValuePair) != 2 {
				log.Warnln("Unable to parse key value pair:", splitEnvVarKeyValuePair)
				continue
			}
			if _, ok := checkDeploymentNodeSelectors[parsedEnvVarKeyValuePair[0]]; !ok {
				checkDeploymentNodeSelectors[parsedEnvVarKeyValuePair[0]] = parsedEnvVarKeyValuePair[1]
			}
		}
		log.Infoln("Parsed NODE_SELECTOR:", checkDeploymentNodeSelectors)
	}

	// Parse incoming check pod resource requests and limits
	// Calculated in decimal SI units (15 = 15m cpu).
	millicoreRequest = defaultMillicoreRequest
	if len(millicoreRequestEnv) != 0 {
		cpuRequest, err := strconv.ParseInt(millicoreRequestEnv, 10, 64)
		if err != nil {
			log.Fatalln("error occurred attempting to parse CHECK_POD_CPU_REQUEST:", err)
		}
		millicoreRequest = int(cpuRequest)
		log.Infoln("Parsed CHECK_POD_CPU_REQUEST:", millicoreRequest)
	}

	// Calculated in decimal SI units (75 = 75m cpu).
	millicoreLimit = defaultMillicoreLimit
	if len(millicoreLimitEnv) != 0 {
		cpuLimit, err := strconv.ParseInt(millicoreLimitEnv, 10, 64)
		if err != nil {
			log.Fatalln("error occurred attempting to parse CHECK_POD_CPU_LIMIT:", err)
		}
		millicoreLimit = int(cpuLimit)
		log.Infoln("Parsed CHECK_POD_CPU_LIMIT:", millicoreLimit)
	}

	// Calculated in binary SI units (20 * 1024^2 = 20Mi memory).
	memoryRequest = defaultMemoryRequest
	if len(memoryRequestEnv) != 0 {
		memRequest, err := strconv.ParseInt(memoryRequestEnv, 10, 64)
		if err != nil {
			log.Fatalln("error occurred attempting to parse CHECK_POD_MEM_REQUEST:", err)
		}
		memoryRequest = int(memRequest) * 1024 * 1024
		log.Infoln("Parsed CHECK_POD_MEM_REQUEST:", memoryRequest)
	}

	// Calculated in binary SI units (75 * 1024^2 = 75Mi memory).
	memoryLimit = defaultMemoryLimit
	if len(memoryLimitEnv) != 0 {
		memLimit, err := strconv.ParseInt(memoryLimitEnv, 10, 64)
		if err != nil {
			log.Fatalln("error occurred attempting to parse CHECK_POD_MEM_LIMIT:", err)
		}
		memoryLimit = int(memLimit) * 1024 * 1024
		log.Infoln("Parsed CHECK_POD_MEM_LIMIT:", memoryLimit)
	}

	// Parse incoming check service account
	checkServiceAccount = defaultCheckServieAccount
	if len(checkServiceAccountEnv) != 0 {
		checkServiceAccount = checkServiceAccountEnv
		log.Infoln("Parsed CHECK_SERVICE_ACCOUNT:", checkServiceAccount)
	}

	// Set check time limit to default
	checkTimeLimit = defaultCheckTimeLimit
	// Get the deadline time in unix from the env var
	timeDeadline, err := kh.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}
	checkTimeLimit = timeDeadline.Sub(time.Now().Add(time.Second * 5))
	log.Infoln("Check time limit set to:", checkTimeLimit)

	// Parse incoming deployment rolling-update environment variable
	if len(rollingUpdateEnv) != 0 {
		var err error
		rollingUpdate, err = strconv.ParseBool(rollingUpdateEnv)
		if err != nil {
			log.Fatalln("Failed to parse rolling-update boolean variable:", err)
		}
	}
	log.Infoln("Parsed CHECK_DEPLOYMENT_ROLLING_UPDATE:", rollingUpdate)
	if rollingUpdate {
		if checkImageURL == checkImageURLB {
			log.Infoln("The same container image cannot be used for the rolling-update check.  Using default images.")
			checkImageURL = defaultCheckImageURL
			checkImageURLB = defaultCheckImageURLB
			log.Infoln("Setting initial container image to:", checkImageURL)
			log.Infoln("Setting update container image to:", checkImageURLB)
		}
		log.Infoln("Check deployment image will be rolled from [" + checkImageURL + "] to [" + checkImageURLB + "]")
	}

	// Parse incoming container environment variables
	// (in case custom used images require additional environment variables)
	if len(additionalEnvVarsEnv) != 0 {
		splitEnvVars := strings.Split(additionalEnvVarsEnv, ",")
		for _, splitEnvVarKeyValuePair := range splitEnvVars {
			parsedEnvVarKeyValuePair := strings.Split(splitEnvVarKeyValuePair, "=")
			if _, ok := additionalEnvVars[parsedEnvVarKeyValuePair[0]]; !ok {
				additionalEnvVars[parsedEnvVarKeyValuePair[0]] = parsedEnvVarKeyValuePair[1]
			}
		}
		log.Infoln("Parsed ADDITIONAL_ENV_VARS:", additionalEnvVars)
	}

	// Parse incoming custom shutdown grace period seconds
	shutdownGracePeriod = defaultShutdownGracePeriod
	if len(shutdownGracePeriodEnv) != 0 {
		duration, err := time.ParseDuration(shutdownGracePeriodEnv)
		if err != nil {
			log.Fatalln("error occurred attempting to parse SHUTDOWN_GRACE_PERIOD:", err)
		}
		if duration.Seconds() < 1 {
			log.Fatalln("error occurred attempting to parse SHUTDOWN_GRACE_PERIOD.  A value less than 1 was parsed:", duration.Seconds())
		}
		shutdownGracePeriod = duration
		log.Infoln("Parsed SHUTDOWN_GRACE_PERIOD:", shutdownGracePeriod)
	}
}
