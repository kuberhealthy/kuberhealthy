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

// Package masterCalculation determines the master pod in multi pod
// kuberhealthy deployments
package masterCalculation // import "github.com/Comcast/kuberhealthy/v2/pkg/masterCalculation"

import (
	"errors"
	"os"
	"sort"
	"strings"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var namespace = os.Getenv("POD_NAMESPACE")
var enableForceMaster bool // indicates we should always report as master for debugging

// DebugAlwaysMasterOn makes all master queries return true without logic
func DebugAlwaysMasterOn() {
	enableForceMaster = true
}

// EnableDebug enables debug logging
func EnableDebug() {
	log.SetLevel(log.DebugLevel)
}

// getEnvVar attempts to retrieve and then validates an environmental variable
func getEnvVar(v string) (string, error) {
	var err error
	envVar := os.Getenv(v)
	if len(envVar) < 1 {
		err = errors.New("Could not retrieve environment variable, or it had no content. " + v)
		return envVar, err
	}
	return envVar, err
}

// CalculateMaster determines which kuberhealthy pod should assume the master role
func CalculateMaster(client *kubernetes.Clientset) (string, error) {

	log.Debugln("Calculating current master...")

	// get a list of all kuberhealthy pods
	pods, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: "app=kuberhealthy", FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return "", err
	}

	// create a slice of all kuberhealthy pod names for use in sort
	var podlist []string
	for _, p := range pods.Items {
		podlist = append(podlist, p.Name)
	}

	if len(podlist) < 1 {
		return "", errors.New("Failed to retrieve list of Kuberhealthy pods")
	}

	// choose master by grabbing the first in alphabetical order based on
	// the pod name
	sort.Strings(podlist)
	master := podlist[0]

	log.Debugln("Calculated master as", master)
	return master, err
}

// IAmMaster determines if the executing pod is the cluster master or not
func IAmMaster(client *kubernetes.Clientset) (bool, error) {

	// if we are in debug enable master always, then just return true
	if enableForceMaster {
		return true, nil
	}

	master, err := CalculateMaster(client)
	if err != nil {
		return false, err
	}

	// get name of the pod running this check from an environment variable we set
	// in the pod spec
	myPod, err := getEnvVar("POD_NAME")
	log.Debugln("My pod hostname is: " + myPod)
	if err != nil {
		log.Errorln(err)
	}

	// if our pod name matches the calculated master pod name, we are the master
	if strings.ToLower(myPod) == strings.ToLower(master) {
		log.Debugln("I am master")
		return true, err
	}

	log.Debugln("I am NOT master")
	return false, err
}
