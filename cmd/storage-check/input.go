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
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
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
	checkStorageImage = defaultCheckStorageImage
	if len(checkStorageImageEnv) != 0 {
		checkStorageImage = checkStorageImageEnv
		log.Infoln("Parsed CHECK_STORAGE_IMAGE:", checkStorageImage)
	}

	// Parse incoming check init image environment variable.
	checkStorageInitImage = defaultCheckStorageInitImage
	if len(checkStorageInitImageEnv) != 0 {
		checkStorageInitImage = checkStorageInitImageEnv
		log.Infoln("Parsed CHECK_STORAGE_INIT_IMAGE:", checkStorageInitImage)
	}

	// Parse incoming check storage name.
	checkStorageName = defaultCheckStorageName
	log.Infof("input.go checkStorageName default=%s env=%s", checkStorageName, checkStorageNameEnv)
	if len(checkStorageNameEnv) != 0 {
		checkStorageName = checkStorageNameEnv
		log.Infoln("Parsed CHECK_STORAGE_NAME:", checkStorageName)
	}

	// Parse incoming check storage init job name.
	checkStorageInitJobName = defaultCheckStorageInitJobName
	log.Infof("input.go checkStorageInitJobName default=%s env=%s", checkStorageInitJobName, checkStorageInitJobNameEnv)
	if len(checkStorageInitJobNameEnv) != 0 {
		checkStorageInitJobName = checkStorageInitJobNameEnv
		log.Infoln("Parsed CHECK_STORAGE_INIT_JOB_NAME:", checkStorageInitJobName)
	}

	// Parse incoming check storage job name.
	checkStorageJobName = defaultCheckStorageJobName
	log.Infof("input.go checkStorageJobName default=%s env=%s", checkStorageJobName, checkStorageJobNameEnv)
	if len(checkStorageJobNameEnv) != 0 {
		checkStorageJobName = checkStorageJobNameEnv
		log.Infoln("Parsed CHECK_STORAGE_INIT_JOB_NAME:", checkStorageJobName)
	}

	// Parse incoming init storage command.
	checkStorageInitCommand = defaultCheckStorageInitCommand
	log.Infof("input.go checkStorageInitCommand default=%s env=%s", checkStorageInitCommand, checkStorageInitCommandEnv)
	if len(checkStorageInitCommandEnv) != 0 {
		checkStorageInitCommand = checkStorageInitCommandEnv
		log.Infoln("Parsed CHECK_STORAGE_INIT_COMMAND:", checkStorageInitCommand)
	}

	// Parse incoming check storage init command args.
	checkStorageInitCommandArgs = defaultCheckStorageInitCommandArgs
	log.Infof("input.go checkStorageInitCommandArgs default=%s env=%s", checkStorageInitCommandArgs, checkStorageInitCommandArgsEnv)
	if len(checkStorageInitCommandArgsEnv) != 0 {
		checkStorageInitCommandArgs = checkStorageInitCommandArgsEnv
		log.Infoln("Parsed CHECK_STORAGE_COMMAND_ARGS:", checkStorageInitCommandArgs)
	}

	// Parse incoming check storage command.
	checkStorageCommand = defaultCheckStorageCommand
	log.Infof("input.go checkStorageCommand default=%s env=%s", checkStorageCommand, checkStorageCommandEnv)
	if len(checkStorageCommandEnv) != 0 {
		checkStorageCommand = checkStorageCommandEnv
		log.Infoln("Parsed CHECK_STORAGE_COMMAND:", checkStorageCommand)
	}

	// Parse incoming check storage command args.
	checkStorageCommandArgs = defaultCheckStorageCommandArgs
	log.Infof("input.go checkStorageCommandArgs default=%s env=%s", checkStorageCommandArgs, checkStorageCommandArgsEnv)
	if len(checkStorageCommandArgsEnv) != 0 {
		checkStorageCommandArgs = checkStorageCommandArgsEnv
		log.Infoln("Parsed CHECK_STORAGE_COMMAND_ARGS:", checkStorageCommandArgs)
	}

	// Parse incoming PVC Size
	pvcSize = defaultPvcSize
	log.Infof("input.go pvcSize default=%s env=%s", pvcSize, pvcSizeEnv)
	if len(pvcSizeEnv) != 0 {
		pvcSize = pvcSizeEnv
		log.Infoln("Parsed CHECK_STORAGE_PVC_SIZE:", pvcSize)
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

	// Set check time limit to default
	checkTimeLimit = defaultCheckTimeLimit
	// Get the deadline time in unix from the env var
	timeDeadline, err := kh.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}
	checkTimeLimit = timeDeadline.Sub(time.Now().Add(time.Second * 5))
	log.Infoln("Check time limit set to:", checkTimeLimit)

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
