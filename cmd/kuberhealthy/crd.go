package main

import (
	"errors"
	"strings"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	khjobv1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khjob/v1"
	khstatev1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khstate/v1"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external"
)

// setCheckStateResource puts a check state's state into the specified CRD resource.  It sets the AuthoritativePod
// to the server's hostname and sets the LastUpdate time to now.
func setCheckStateResource(checkName string, checkNamespace string, state khstatev1.WorkloadDetails) error {

	name := sanitizeResourceName(checkName)

	// we must fetch the existing state to use the current resource version
	// int found within
	existingState, err := khStateClient.KuberhealthyStates(checkNamespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return errors.New("Error retrieving CRD for: " + name + " " + err.Error())
	}
	resourceVersion := existingState.GetResourceVersion()

	// set the pod name that wrote the khstate
	state.AuthoritativePod = podHostname
	now := metav1.Now() // set the time the khstate was last
	state.LastRun = &now

	khState := khstatev1.NewKuberhealthyState(name, state)
	khState.SetResourceVersion(resourceVersion)
	// TODO - if "try again" message found in error, then try again

	log.Debugln(checkNamespace, checkName, "writing khstate with ok:", state.OK, "and errors:", state.Errors, "at last run:", state.LastRun)
	_, err = khStateClient.KuberhealthyStates(checkNamespace).Update(&khState)
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
func ensureStateResourceExists(checkName string, checkNamespace string, workload khstatev1.KHWorkload) error {
	name := sanitizeResourceName(checkName)

	log.Debugln("Checking existence of custom resource:", name)
	state, err := khStateClient.KuberhealthyStates(checkNamespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found") {
			log.Infoln("Custom resource not found, creating resource:", name, " - ", err)
			initialDetails := khstatev1.NewWorkloadDetails(workload)
			initialState := khstatev1.NewKuberhealthyState(name, initialDetails)
			_, err := khStateClient.KuberhealthyStates(checkNamespace).Create(&initialState)
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
func getCheckState(c *external.Checker) (khstatev1.WorkloadDetails, error) {

	var state = khstatev1.NewWorkloadDetails(khstatev1.KHCheck)
	var err error
	name := sanitizeResourceName(c.Name())

	// make sure the CRD exists, even when checking status
	err = ensureStateResourceExists(c.Name(), c.CheckNamespace(), khstatev1.KHCheck)
	if err != nil {
		return state, errors.New("Error validating CRD exists: " + name + " " + err.Error())
	}

	log.Debugln("Retrieving khstate custom resource for:", name)
	khstate, err := khStateClient.KuberhealthyStates(c.CheckNamespace()).Get(name, metav1.GetOptions{})
	if err != nil {
		return state, errors.New("Error retrieving custom khstate resource: " + name + " " + err.Error())
	}
	log.Debugln("Successfully retrieved khstate resource:", name)
	return khstate.Spec, nil
}

// getCheckState retrieves the check values from the kuberhealthy khstate
// custom resource
func getJobState(j *external.Checker) (khstatev1.WorkloadDetails, error) {

	var state = khstatev1.NewWorkloadDetails(khstatev1.KHJob)
	var err error
	name := sanitizeResourceName(j.Name())

	// make sure the CRD exists, even when checking status
	err = ensureStateResourceExists(j.Name(), j.CheckNamespace(), khstatev1.KHJob)
	if err != nil {
		return state, errors.New("Error validating CRD exists: " + name + " " + err.Error())
	}

	log.Debugln("Retrieving khstate custom resource for:", name)
	khstate, err := khStateClient.KuberhealthyStates(j.CheckNamespace()).Get(name, metav1.GetOptions{})
	if err != nil {
		return state, errors.New("Error retrieving custom khstate resource: " + name + " " + err.Error())
	}
	log.Debugln("Successfully retrieved khstate resource:", name)
	return khstate.Spec, nil
}

// setJobPhase updates the kuberhealthy job phase depending on the state of its run.
func setJobPhase(jobName string, jobNamespace string, jobPhase khjobv1.JobPhase) error {

	kj, err := khJobClient.KuberhealthyJobs(jobNamespace).Get(jobName, metav1.GetOptions{})
	if err != nil {
		log.Errorln("error getting khjob:", jobName, err)
		return err
	}
	resourceVersion := kj.GetResourceVersion()
	updatedJob := khjobv1.NewKuberhealthyJob(jobName, jobNamespace, kj.Spec)
	updatedJob.SetResourceVersion(resourceVersion)
	log.Infoln("Setting khjob phase to:", jobPhase)
	updatedJob.Spec.Phase = jobPhase

	_, err = khJobClient.KuberhealthyJobs(jobNamespace).Update(&updatedJob)
	return err
}
