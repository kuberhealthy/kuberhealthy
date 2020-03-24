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
	"errors"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Comcast/kuberhealthy/v2/pkg/health"
	"github.com/Comcast/kuberhealthy/v2/pkg/khstatecrd"
)

// setCheckStateResource puts a check state's state into the specified CRD resource.  It sets the AuthoritativePod
// to the server's hostname and sets the LastUpdate time to now.
func setCheckStateResource(checkName string, checkNamespace string, state health.CheckDetails) error {

	name := sanitizeResourceName(checkName)

	// we must fetch the existing state to use the current resource version
	// int found within
	existingState, err := khStateClient.Get(metav1.GetOptions{}, stateCRDResource, name, checkNamespace)
	if err != nil {
		return errors.New("Error retrieving CRD for: " + name + " " + err.Error())
	}
	resourceVersion := existingState.GetResourceVersion()

	// set the pod name that wrote the khstate
	state.AuthoritativePod = podHostname
	state.LastRun = time.Now() // set the time the khstate was last

	khState := khstatecrd.NewKuberhealthyState(name, state)
	khState.SetResourceVersion(resourceVersion)
	// TODO - if "try again" message found in error, then try again

	log.Debugln(checkNamespace, checkName, "writing khstate with ok:", state.OK, "and errors:", state.Errors, "at last run:", state.LastRun)
	_, err = khStateClient.Update(&khState, stateCRDResource, name, checkNamespace)
	return err
}

// sanitizeResourceName cleans up the check names for use in CRDs.
// DNS-1123 subdomains must consist of lower case alphanumeric characters, '-'
// or '.', and must start and end with an alphanumeric character (e.g.
// 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?
// (\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')
func sanitizeResourceName(c string) string {

	// the name we pass to the CRD must be lowercase
	nameLower := strings.ToLower(c)
	return strings.Replace(nameLower, " ", "-", -1)
}

// ensureStateResourceExists checks for the existence of the specified resource and creates it if it does not exist
func ensureStateResourceExists(checkName string, checkNamespace string) error {
	name := sanitizeResourceName(checkName)

	log.Debugln("Checking existence of custom resource:", name)
	state, err := khStateClient.Get(metav1.GetOptions{}, stateCRDResource, name, checkNamespace)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Infoln("Custom resource not found, creating resource:", name, " - ", err)
			initialDetails := health.NewCheckDetails()
			initialState := khstatecrd.NewKuberhealthyState(name, initialDetails)
			_, err := khStateClient.Create(&initialState, stateCRDResource, checkNamespace)
			if err != nil {
				return errors.New("Error creating custom resource: " + name + ": " + err.Error())
			}
		} else {
			return err
		}
	}
	if state.Spec.Errors != nil {
		log.Debugln("khstate custom resource found:", name)
	}
	return nil
}

// getCheckState retrieves the check values from the kuberhealthy khstate
// custom resource
func getCheckState(c KuberhealthyCheck) (health.CheckDetails, error) {

	var state = health.NewCheckDetails()
	var err error
	name := sanitizeResourceName(c.Name())

	// make sure the CRD exists, even when checking status
	err = ensureStateResourceExists(c.Name(), c.CheckNamespace())
	if err != nil {
		return state, errors.New("Error validating CRD exists: " + name + " " + err.Error())
	}

	log.Debugln("Retrieving khstate custom resource for:", name)
	khstate, err := khStateClient.Get(metav1.GetOptions{}, stateCRDResource, name, c.CheckNamespace())
	if err != nil {
		return state, errors.New("Error retrieving custom khstate resource: " + name + " " + err.Error())
	}
	log.Debugln("Successfully retrieved khstate resource:", name)
	return khstate.Spec, nil
}
