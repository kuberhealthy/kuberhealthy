package main

import (
	"context"
	"errors"
	"strings"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	khcrds "github.com/kuberhealthy/crds/api/v1"
	"github.com/kuberhealthy/kuberhealthy/v3/pkg/checks/external"
)

// setCheckStateResource puts a check state's state into the specified CRD resource.  It sets the AuthoritativePod
// to the server's hostname and sets the LastUpdate time to now.
func setCheckStateResource(ctx context.Context, checkName string, checkNamespace string, state khcrds.KuberhealthyState) error {

	name := sanitizeResourceName(checkName)

	// we must fetch the existing state to use the current resource version
	// int found within
	existingState, err := KubernetesClient.GetKuberhealthyState(checkName, checkNamespace)
	if err != nil {
		return errors.New("Error retrieving CRD for: " + name + " " + err.Error())
	}
	resourceVersion := existingState.GetResourceVersion()

	// set the pod name that wrote the khstate
	state.Spec.AuthoritativePod = podHostname
	now := metav1.Now() // set the time the khstate was last
	state.Spec.LastRun = &now

	state.Name = name
	state.Namespace = checkNamespace
	state.SetResourceVersion(resourceVersion)

	log.Debugln(checkNamespace, checkName, "writing khstate with ok:", state.Spec.OK, "and errors:", state.Spec.Errors, "at last run:", state.Spec.LastRun)
	err = KubernetesClient.UpdateKuberhealthyState(&state)
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
func ensureStateResourceExists(ctx context.Context, checkName string, checkNamespace string) error {
	name := sanitizeResourceName(checkName)

	log.Debugln("Checking existence of custom resource:", name)
	state, err := KubernetesClient.GetKuberhealthyState(checkName, checkNamespace)
	if err != nil {
		if k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found") {
			log.Infoln("Custom resource not found, creating resource:", name, " - ", err)
			newKHState := &khcrds.KuberhealthyState{
				TypeMeta: metav1.TypeMeta{
					Kind:       "",
					APIVersion: "v1",
				},
				Spec: khcrds.KuberhealthyStateSpec{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      checkName,
					Namespace: checkNamespace,
				},
			}
			err = KubernetesClient.CreateKuberhealthyState(newKHState)
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

// getCheckState retrieves the check values from the kuberhealthy khstate custom resource
func getCheckState(ctx context.Context, c *external.Checker) (*khcrds.KuberhealthyState, error) {

	var err error
	name := sanitizeResourceName(c.Name())

	// make sure the CRD exists, even when checking status
	err = ensureStateResourceExists(ctx, c.Name(), c.CheckNamespace())
	if err != nil {
		return nil, errors.New("Error validating CRD exists: " + name + " " + err.Error())
	}

	log.Debugln("Retrieving khstate custom resource for:", name)

	khstate, err := KubernetesClient.GetKuberhealthyState(name, c.Namespace)
	if err != nil {
		return nil, errors.New("Error retrieving custom khstate resource: " + name + " " + err.Error())
	}
	log.Debugln("Successfully retrieved khstate resource:", name)
	return khstate, nil
}

// getCheckState retrieves the check values from the kuberhealthy khstate custom resource
func getJobState(ctx context.Context, j *external.Checker) (*khcrds.KuberhealthyState, error) {

	// var state = khstatev1.NewWorkloadDetails(khstatev1.KHJob)
	var err error
	name := sanitizeResourceName(j.Name())

	// make sure the CRD exists, even when checking status
	err = ensureStateResourceExists(ctx, j.Name(), j.CheckNamespace())
	if err != nil {
		return nil, errors.New("Error validating CRD exists: " + name + " " + err.Error())
	}

	log.Debugln("Retrieving khstate custom resource for:", name)
	khstate, err := KubernetesClient.GetKuberhealthyState(name, j.Namespace)
	if err != nil {
		return khstate, errors.New("Error retrieving custom khstate resource: " + name + " " + err.Error())
	}
	log.Debugln("Successfully retrieved khstate resource:", name)
	return khstate, nil
}

// setJobPhase updates the kuberhealthy job phase depending on the state of its run.
func setJobPhase(ctx context.Context, jobName string, jobNamespace string, jobPhase khcrds.JobPhase) error {

	kj, err := KubernetesClient.GetKuberhealthyJob(jobName, jobNamespace)
	if err != nil {
		log.Errorln("error getting khjob:", jobName, err)
		return err
	}
	kj.Spec.Phase = jobPhase
	log.Infoln("Setting khjob phase to:", jobPhase)

	return KubernetesClient.UpdateKuberhealthyJob(kj)
}
