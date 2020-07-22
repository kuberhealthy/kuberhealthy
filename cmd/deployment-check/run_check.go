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
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

// runDeploymentCheck sets up a deployment and applies it to the cluster.
// Sets up a deployment and service and uses the client to deploy the test deployment and service.
// Attempts to hit the service hostname endpoint, looking for a 200 and reports a success if able to retrieve a 200.
func runDeploymentCheck() {

	log.Infoln("Starting check.")

	// Init a timeout for this entire check run.
	runTimeout := time.After(checkTimeLimit)

	// Init a timeout for cleaning up the check.  Assume that the check should not take more than 2m.
	cleanupTimeout := time.After(time.Minute * 2)

	// TODO: Update this logic to unique services and deployments
	// Delete all check resources (deployment & service) from this check that should not exist.
	select {
	case err := <-cleanUpOrphanedResources(ctx):
		// If the clean up completes with errors, we report those and stop the check cleanly.
		if err != nil {
			log.Errorln("error when cleaning up resources:", err)
			reportErrorsToKuberhealthy([]string{err.Error()})
			return
		}
		log.Infoln("Successfully cleaned up prior check resources.")
	case <-ctx.Done():
		// If there is a cancellation interrupt signal.
		log.Infoln("Canceling cleanup and shutting down from interrupt.")
		reportErrorsToKuberhealthy([]string{"failed to perform pre-check cleanup within timeout"})
		return
	case <-cleanupTimeout:
		// If the clean up took too long, exit.
		reportErrorsToKuberhealthy([]string{"failed to perform pre-check cleanup within timeout"})
		return
	}

	// Create a deployment resource.
	deploymentConfig := createDeploymentConfig(checkImageURL)
	log.Infoln("Created deployment resource.")

	// Apply the deployment struct manifest to the cluster.
	var deploymentResult DeploymentResult
	select {
	case deploymentResult = <-createDeployment(deploymentConfig):
		// Handle errors when the deployment creation process completes.
		if deploymentResult.Err != nil {
			log.Errorln("error occurred creating deployment in cluster:", deploymentResult.Err)
			reportErrorsToKuberhealthy([]string{deploymentResult.Err.Error()})
			return
		}
		// Continue with the check if there is no error.
		log.Infoln("Created deployment in", deploymentResult.Deployment.Namespace, "namespace:", deploymentResult.Deployment.Name)
	case <-ctx.Done():
		// If there is a cancellation interrupt signal.
		log.Infoln("Cancelling check and shutting down due to interrupt.")
		reportErrorsToKuberhealthy([]string{"failed to create deployment within timeout"})
		return
	case <-runTimeout:
		// If creating a deployment took too long, exit.
		reportErrorsToKuberhealthy([]string{"failed to create deployment within timeout"})
		return
	}

	// Create a service resource.
	serviceConfig := createServiceConfig(deploymentResult.Deployment.Spec.Template.Labels)
	log.Infoln("Created service resource.")

	// Apply the service struct manifest to the cluster.
	var serviceResult ServiceResult
	select {
	case serviceResult = <-createService(serviceConfig):
		// Handle errors when the service creation process completes.
		if serviceResult.Err != nil {
			log.Errorln("error occurred creating service in cluster:", serviceResult.Err)
			errorReport := []string{serviceResult.Err.Error()} // Make a slice for errors here, because tehre can be more than 1 error.
			// Clean up the check. A deployment and service was brought up, but could not get a 200 OK from requests.
			cleanUpError := cleanUp(ctx)
			if cleanUpError != nil {
				errorReport = append(errorReport, cleanUpError.Error())
			}
			reportErrorsToKuberhealthy(errorReport)
			return
		}
		// Continue with the check if there is no error.
		log.Infoln("Created service in", serviceResult.Service.ObjectMeta.Namespace, "namespace:", serviceResult.Service.ObjectMeta.Name)
	case <-ctx.Done():
		// If there is a cancellation interrupt signal, exit.
		log.Infoln("Cancelling check and shutting down due to interrupt.")
		reportErrorsToKuberhealthy([]string{"failed to create service within timeout"})
		return
	case <-runTimeout:
		// If creating a service took too long, exit.
		reportErrorsToKuberhealthy([]string{"failed to create service within timeout"})
		return
	}

	// Get an ingress hostname associated with the service.
	// hostname := getServiceLoadBalancerHostname()
	ipAddress := getServiceClusterIP()
	if len(ipAddress) == 0 {
		// If the retrieved address is empty or nil, clean up and exit.
		log.Infoln("Cleaning up check and exiting because the cluster IP is nil: ", ipAddress)
		errorReport := []string{} // Make a slice for errors here, because there can be more than 1 error.
		// Clean up the check. A deployment was brought up, but no ingress was created.
		cleanUpError := cleanUp(ctx)
		if cleanUpError != nil {
			errorReport = append(errorReport, cleanUpError.Error())
		}
		// hostnameError := fmt.Errorf("service load balancer ingress hostname is nil: %s", hostname)
		addressError := fmt.Errorf("service cluster IP address is nil: %s", ipAddress)
		// Report errors to Kuberhealthy and exit.
		errorReport = append(errorReport, addressError.Error())
		reportErrorsToKuberhealthy(errorReport)
		return
	}

	// Make an HTTP request to the load balancer for the service at the external IP address.
	// Utilize a backoff loop for the request, the hostname needs to be allotted enough time
	// for the hostname to resolve and come up.
	select {
	case err := <-makeRequestToDeploymentCheckService(ipAddress):
		if err != nil {
			// Handle errors when the HTTP request process completes.
			log.Errorln("error occurred making request to service in cluster:", err)
			errorReport := []string{err.Error()} // Make a slice for errors here, because tehre can be more than 1 error.
			// Clean up the check. A deployment and service was brought up, but could not get a 200 OK from requests.
			cleanUpError := cleanUp(ctx)
			if cleanUpError != nil {
				errorReport = append(errorReport, cleanUpError.Error())
			}
			reportErrorsToKuberhealthy(errorReport)
			return
		}
		// Continue with the check if there is no error.
		log.Infoln("Successfully hit service endpoint.")
	case <-ctx.Done():
		// If there is a cancellation interrupt signal, exit.
		log.Infoln("Cancelling check and shutting down due to interrupt.")
		reportErrorsToKuberhealthy([]string{"failed to make http request to the deployment service cluster IP at " + ipAddress + " within timeout"})
		return
	case <-runTimeout:
		// If requests to the hostname endpoint for a status code of 200 took too long, exit.
		reportErrorsToKuberhealthy([]string{"failed to make http request to the deployment service cluster IP at " + ipAddress + " within timeout"})
		return
	}

	// If a rolling update is enabled, perform a rolling update on the service.
	if rollingUpdate {

		log.Infoln("Rolling update option is enabled. Performing roll.")

		// Create a rolling-update deployment resource.
		rolledUpdateConfig := createDeploymentConfig(checkImageURLB)
		log.Infoln("Created rolling-update deployment resource.")

		// Apply the deployment struct manifest to the cluster.
		var updateDeploymentResult DeploymentResult
		select {
		case updateDeploymentResult = <-updateDeployment(rolledUpdateConfig):
			// Handle errors when the deployment creation process completes.
			if updateDeploymentResult.Err != nil {
				log.Errorln("error occurred applying rolling-update to deployment in cluster:", updateDeploymentResult.Err)
				errorReport := []string{updateDeploymentResult.Err.Error()} // Make a slice for errors here, because tehre can be more than 1 error.
				// Clean up the check. A deployment and service was brought up, but could not get a 200 OK from requests.
				cleanUpError := cleanUp(ctx)
				if cleanUpError != nil {
					errorReport = append(errorReport, cleanUpError.Error())
				}
				reportErrorsToKuberhealthy(errorReport)
				return
			}
			// Continue with the check if there is no error.
			log.Infoln("Rolled deployment in", updateDeploymentResult.Deployment.Namespace, "namespace:", updateDeploymentResult.Deployment.Name)
		case <-ctx.Done():
			// If there is a cancellation interrupt signal.
			log.Infoln("Cancelling check and shutting down due to interrupt.")
			reportErrorsToKuberhealthy([]string{"failed to update deployment " + deploymentResult.Deployment.Name + " within timeout"})
			return
		case <-runTimeout:
			// If creating a deployment took too long, exit.
			reportErrorsToKuberhealthy([]string{"failed to update deployment " + deploymentResult.Deployment.Name + " within timeout"})
			return
		}

		// Hit the service again, looking for a 200.
		select {
		case err := <-makeRequestToDeploymentCheckService(ipAddress):
			// Handle errors when the HTTP request process completes.
			if err != nil {
				log.Errorln("error occurred creating service in cluster:", err)
				errorReport := []string{err.Error()} // Make a slice for errors here, because tehre can be more than 1 error.
				// Clean up the check. A deployment and service was brought up, but could not get a 200 OK from requests.
				cleanUpError := cleanUp(ctx)
				if cleanUpError != nil {
					errorReport = append(errorReport, cleanUpError.Error())
				}
				reportErrorsToKuberhealthy([]string{err.Error()})
				return
			}
			// Continue with the check if there is no error.
			log.Infoln("Successfully hit service endpoint after rolling-update.")
		case <-ctx.Done():
			// If there is a cancellation interrupt signal, exit.
			log.Infoln("Cancelling check and shutting down due to interrupt.")
			reportErrorsToKuberhealthy([]string{"failed to make http request to the deployment service cluster IP at " + ipAddress + " within timeout"})
			return
		case <-runTimeout:
			// If requests to the hostname endpoint for a status code of 200 took too long, exit.
			reportErrorsToKuberhealthy([]string{"failed to make http request to the deployment service cluster IP at " + ipAddress + " within timeout"})
			return
		}
	}

	// Clean up!
	cleanUpError := cleanUp(ctx)
	if cleanUpError != nil {
		reportErrorsToKuberhealthy([]string{cleanUpError.Error()})
	}
	// Report to Kuberhealthy.
	reportOKToKuberhealthy()
}

// cleanUp cleans up the deployment check and all resource manifests created that relate to
// the check.
// TODO - add in context that expires when check times out
func cleanUp(ctx context.Context) error {

	log.Infoln("Cleaning up deployment and service.")
	var err error
	var resultErr error
	errorMessage := ""

	// Delete the service.
	// TODO - add select to catch context timeout expiration
	err = deleteServiceAndWait(ctx)
	if err != nil {
		log.Errorln("error cleaning up service:", err)
		errorMessage = errorMessage + "error cleaning up service:" + err.Error()
	}

	// Delete the deployment.
	// TODO - add select to catch context timeout expiration
	err = deleteDeploymentAndWait(ctx)
	if err != nil {
		log.Errorln("error cleaning up deployment:", err)
		if len(errorMessage) != 0 {
			errorMessage = errorMessage + " | "
		}
		errorMessage = errorMessage + "error cleaning up deployment:" + err.Error()
	}

	log.Infoln("Finished clean up process.")

	// Create an error if errors occurred during the clean up process.
	if len(errorMessage) != 0 {
		resultErr = fmt.Errorf("%s", errorMessage)
	}

	return resultErr
}

// cleanUpOrphanedResources cleans up previous deployment and services and ensures
// a clean slate before beginning a deployment and service check.
func cleanUpOrphanedResources(ctx context.Context) chan error {

	cleanUpChan := make(chan error)

	go func(c context.Context) {
		log.Infoln("Wiping all found orphaned resources belonging to this check.")

		defer close(cleanUpChan)

		svcExists, err := findPreviousService()
		if err != nil {
			log.Warnln("Failed to find previous service:", err.Error())
		}
		if svcExists {
			log.Infoln("Found previous service.")
		}

		deploymentExists, err := findPreviousDeployment()
		if err != nil {
			log.Warnln("Failed to find previous deployment:", err.Error())
		}
		if deploymentExists {
			log.Infoln("Found previous deployment.")
		}

		if svcExists || deploymentExists {
			cleanUpChan <- cleanUp(c)
		} else {
			cleanUpChan <- nil
		}
	}(ctx)

	return cleanUpChan
}
