// Copyright 2018 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
			log.Fatalln("error occured attempting to parse CHECK_CONTAINER_PORT:", err)
		}
		checkContainerPort = int32(port)
		log.Infoln("Parsed CHECK_CONTAINER_PORT:", checkContainerPort)
	}

	// Parse incoming load balancer port environment variable
	checkLoadBalancerPort = defaultCheckLoadBalancerPort
	if len(checkLoadBalancerPortEnv) != 0 {
		port, err := strconv.Atoi(checkLoadBalancerPortEnv)
		if err != nil {
			log.Fatalln("error occured attempting to parse CHECK_LOAD_BALANCER_PORT:", err)
		}
		checkLoadBalancerPort = int32(port)
		log.Infoln("Parsed CHECK_LOAD_BALANCER_PORT:", checkLoadBalancerPort)
	}

	// Parse incoming namespace environment variable
	checkNamespace = defaultCheckNamespace
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

	// Parse incoming check service account
	checkServiceAccount = defaultCheckServieAccount
	if len(checkServiceAccountEnv) != 0 {
		checkServiceAccount = checkServiceAccountEnv
		log.Infoln("Parsed CHECK_SERVICE_ACCOUNT:", checkServiceAccount)
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
