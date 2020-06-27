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
	"strings"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
)

// getHostname attempts to determine the hostname this program is running on
func getHostname() string {
	defaultHostname := "kuberhealthy"
	host, err := os.Hostname()
	if len(host) == 0 || err != nil {
		log.Warningln("Unable to determine hostname! Using default placeholder:", defaultHostname)
		return defaultHostname // default if no hostname can be found
	}
	return strings.ToLower(host)
}

// formatNodes formats string list into readable string for logging and error message purposes
func formatNodes(nodeList []string) string {
	if len(nodeList) > 0 {
		return strings.Join(nodeList, ", ")
	}
	return ""
}

// getDSPodsNodeList transforms podList to a list of pod node name strings. Used for error messaging.
func getDSPodsNodeList(podList *apiv1.PodList) string {

	var nodeList []string
	if len(podList.Items) != 0 {
		for _, p := range podList.Items {
			nodeList = append(nodeList, p.Spec.NodeName)
		}
	}

	return formatNodes(nodeList)
}
