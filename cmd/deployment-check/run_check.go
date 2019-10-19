package main

import (
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
)

// runDeploymentCheck sets up a deployment and applies it to the cluster.
// Sets up a deployment and service and uses the client to deploy the test deployment and service.
// Attempts to hit the service hostname endpoint, looking for a 200 and reports a success if able to retrieve a 200.
func runDeploymentCheck() error {

	log.Infoln("Starting check.")

	// Exit the check if the k8s client is nil.
	if client == nil {
		return errors.New("nil kubernetes client passed to deployment check")
	}

	// Init a timeout for this entire check run.
	runTimeout := time.After(checkTimeLimit)

	// Init a timeout for cleaning up the check.  Assume that the check should not take more than 2m.
	cleanupTimeout := time.After(time.Minute * 2)

	// Delete all check resources (deployment & service) from this check that should not exist.
	select {
	case err := <-cleanUpOrphanedResources():
		// If the clean up completes with errors, we report those and stop the check cleanly.
		if err != nil {
			log.Errorln("error when cleaning up resources:", err)
			reportErrorsToKuberhealthy([]string{err.Error()})
			return nil
		}
		log.Infoln("Successfully cleaned up prior check resources.")
	case <-ctx.Done():
		// If there is a cancellation interrupt signal.
		log.Infoln("Canceling cleanup and shutting down from interrupt.")
		return nil
	case <-cleanupTimeout:
		// If the clean up took too long, exit.
		reportErrorsToKuberhealthy([]string{"failed to clean up resources in time"})
		return nil
	}

	// Create a deployment resource.
	deploymentConfig := createDeploymentConfig(false)
	log.Infoln("Created deployment resource.")

	// Apply the deployment struct manifest to the cluster.
	var deploymentResult DeploymentResult
	select {
	case deploymentResult = <-createDeployment(deploymentConfig):
		// Handle errors when the deployment creation process completes.
		if deploymentResult.Err != nil {
			log.Errorln("error occurred creating deployment in cluster:", deploymentResult.Err)
			reportErrorsToKuberhealthy([]string{deploymentResult.Err.Error()})
			return nil
		}
		// Continue with the check if there is no error.
		log.Infoln("Created deployment in", deploymentResult.Deployment.Namespace, "namespace:", deploymentResult.Deployment.Name)
	case <-ctx.Done():
		// If there is a cancellation interrupt signal.
		log.Infoln("Cancelling check and shutting down due to interrupt.")
		return nil
	case <-runTimeout:
		// If creating a deployment took too long, exit.
		reportErrorsToKuberhealthy([]string{"failed to create deployment within timeout"})
		return nil
	}

	// Create a service resource.
	serviceConfig := createServiceConfig(deploymentResult.Deployment.Labels)
	log.Infoln("Created service resource.")

	// Apply the service struct manifest to the cluster.
	var serviceResult ServiceResult
	select {
	case serviceResult = <-createService(serviceConfig):
		// Handle errors when the service creation process completes.
		if serviceResult.Err != nil {
			log.Errorln("error occurred creating service in cluster:", serviceResult.Err)
			reportErrorsToKuberhealthy([]string{serviceResult.Err.Error()})
			return nil
		}
		// Continue with the check if there is no error.
		log.Infoln("Created service in", serviceResult.Service.ObjectMeta.Namespace, "namespace:", serviceResult.Service.ObjectMeta.Name)
	case <-ctx.Done():
		// If there is a cancellation interrupt signal, exit.
		log.Infoln("Cancelling check and shutting down due to interrupt.")
		return nil
	case <-runTimeout:
		// If creating a service took too long, exit.
		reportErrorsToKuberhealthy([]string{"failed to create deployment within timeout"})
		return nil
	}

	// Get an ingress hostname associated with the service.
	hostname := getServiceLoadBalancerHostname()
	if len(hostname) == 0 {
		// If the retrieved address is empty or nil, clean up and exit.
		log.Infoln("Cleaning up check and exiting because the load balancer hostname is nil.")
		errorReport := []string{} // Make a slice for errors here, because there can be more than 1 error.
		// Clean up the check. A deployment was brought up, but no ingress was created.
		cleanUpError := cleanUp()
		if cleanUpError != nil {
			errorReport = append(errorReport, cleanUpError.Error())
		}
		hostnameError := errors.New("service load balancer ingress hostname is nil: " + hostname)
		// Report errors to Kuberhealthy and exit.
		errorReport = append(errorReport, hostnameError.Error())
		reportErrorsToKuberhealthy(errorReport)
		return nil
	}

	// Make an HTTP request to the load balancer for the service at the external IP address.
	// Utilize a backoff loop for the request, the hostname needs to be allotted enough time
	// for the hostname to resolve and come up.
	select {
	case err := <-makeRequestToDeploymentCheckService(hostname):
		if err != nil {
			// Handle errors when the HTTP request process completes.
			log.Errorln("error occurred creating service in cluster:", err)
			reportErrorsToKuberhealthy([]string{err.Error()})
			return nil
		}
		// Continue with the check if there is no error.
		log.Infoln("Successfully hit service endpoint.")
	case <-ctx.Done():
		// If there is a cancellation interrupt signal, exit.
		log.Infoln("Cancelling check and shutting down due to interrupt.")
		return nil
	case <-runTimeout:
		// If requests to the hostname endpoint for a status code of 200 took too long, exit.
		reportErrorsToKuberhealthy([]string{"failed to create deployment within timeout"})
		return nil
	}

	// If a rolling update is enabled, perform a rolling update on the service.
	if rollingUpdate {

		log.Infoln("Rolling update option is enabled.  Performing roll.")

		// Create a rolling-update deployment resource.
		rolledUpdateConfig := createDeploymentConfig(true)
		log.Infoln("Created rolling-update deployment resource.")

		// Apply the deployment struct manifest to the cluster.
		var updateDeploymentResult DeploymentResult
		select {
		case updateDeploymentResult = <-updateDeployment(rolledUpdateConfig):
			// Handle errors when the deployment creation process completes.
			if updateDeploymentResult.Err != nil {
				log.Errorln("error occurred applying rolling-update to deployment in cluster:", updateDeploymentResult.Err)
				reportErrorsToKuberhealthy([]string{updateDeploymentResult.Err.Error()})
				return nil
			}
			// Continue with the check if there is no error.
			log.Infoln("Rolled deployment in", updateDeploymentResult.Deployment.Namespace, "namespace:", updateDeploymentResult.Deployment.Name)
		case <-ctx.Done():
			// If there is a cancellation interrupt signal.
			log.Infoln("Cancelling check and shutting down due to interrupt.")
			return nil
		case <-runTimeout:
			// If creating a deployment took too long, exit.
			reportErrorsToKuberhealthy([]string{"failed to update deployment within timeout"})
			return nil
		}

		// Hit the service again, looking for a 200.
		select {
		case err := <-makeRequestToDeploymentCheckService(hostname):
			if err != nil {
				// Handle errors when the HTTP request process completes.
				log.Errorln("error occurred creating service in cluster:", err)
				reportErrorsToKuberhealthy([]string{err.Error()})
				return nil
			}
			// Continue with the check if there is no error.
			log.Infoln("Successfully hit service endpoint after rolling-update.")
		case <-ctx.Done():
			// If there is a cancellation interrupt signal, exit.
			log.Infoln("Cancelling check and shutting down due to interrupt.")
			return nil
		case <-runTimeout:
			// If requests to the hostname endpoint for a status code of 200 took too long, exit.
			reportErrorsToKuberhealthy([]string{"failed to create deployment within timeout"})
			return nil
		}
	}

	// Clean up!
	cleanUp()

	// Report to Kuberhealthy.
	reportOKToKuberhealthy()
	return nil
}

// cleanUp cleans up the deployment check and all resource manifests created that relate to
// the check.
func cleanUp() error {

	log.Infoln("Cleaning up deployment and service.")
	var err error
	var resultErr error
	errorMessage := ""

	// Delete the service.
	err = deleteService()
	if err != nil {
		log.Errorln("error cleaning up service:", err)
		errorMessage = errorMessage + "error cleaning up service:" + err.Error()
	}

	// Delete the deployment.
	err = deleteDeployment()
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
		resultErr = errors.New(errorMessage)
	}

	return resultErr
}

// cleanUpOrphanedResources cleans up previous deployment and services and ensures
// a clean slate before beginning a deployment and service check.
func cleanUpOrphanedResources() chan error {

	cleanUpChan := make(chan error)

	go func() {
		log.Infoln("Wiping all found orphaned resources belonging to this check.")

		defer close(cleanUpChan)

		// Check if an existing service exists.
		serviceExists, err := findPreviousService()
		if err != nil {
			cleanUpChan <- errors.New("error listing services: " + err.Error())
		}

		// Clean it up if it exists.
		if serviceExists {
			err = cleanUpOrphanedService()
			if err != nil {
				cleanUpChan <- errors.New("error cleaning up old service: " + err.Error())
			}
		}

		// Check if an existing deployment exists.
		deploymentExists, err := findPreviousDeployment()
		if err != nil {
			cleanUpChan <- errors.New("error listing deployments: " + err.Error())
		}

		// Clean it up if it exists.
		if deploymentExists {
			err = cleanUpOrphanedDeployment()
			if err != nil {
				cleanUpChan <- errors.New("error cleaning up old deployment: " + err.Error())
			}
		}
	}()

	return cleanUpChan
}
