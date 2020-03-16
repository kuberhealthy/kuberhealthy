// Copyright 2018 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.package main

package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	watchpkg "k8s.io/apimachinery/pkg/watch"
)

const (
	// Default deployment values.
	defaultLabelKey        = "deployment-timestamp"
	defaultLabelValueBase  = "unix-"
	defaultMinReadySeconds = 5

	// Default deployment strategy values.
	defaultMaxSurge       = 2
	defaultMaxUnavailable = 2

	// Default container values.
	defaultImagePullPolicy = "IfNotPresent"

	// Default container resource requests values.
	defaultMillicoreRequest = 15               // Calculated in decimal SI units (15 = 15m cpu).
	defaultMillicoreLimit   = 75               // Calculated in decimal SI units (75 = 75m cpu).
	defaultMemoryRequest    = 20 * 1024 * 1024 // Calculated in binary SI units (20 * 1024^2 = 20Mi memory).
	defaultMemoryLimit      = 75 * 1024 * 1024 // Calculated in binary SI units (75 * 1024^2 = 75Mi memory).

	// Default container probe values.
	defaultProbeFailureThreshold    = 5  // Number of consecutive failures for the probe to be considered failed (k8s default = 3).
	defaultProbeSuccessThreshold    = 1  // Number of consecutive successes for the probe to be considered successful after having failed (k8s default = 1).
	defaultProbeInitialDelaySeconds = 2  // Number of seconds after container has started before probes are initiated.
	defaultProbeTimeoutSeconds      = 2  // Number of seconds after which the probe times out (k8s default = 1).
	defaultProbePeriodSeconds       = 15 // How often to perform the probe (k8s default = 10).
)

// createDeploymentConfig creates and configures a k8s deployment and returns the struct (ready to apply with client).
func createDeploymentConfig(image string) *v1.Deployment {

	// Make a k8s deployment.
	deployment := &v1.Deployment{}

	// Use a different image if useRollImage is true, to
	checkImage := image

	log.Infoln("Creating deployment resource with", checkDeploymentReplicas, "replica(s) in", checkNamespace, "namespace using image ["+checkImage+"] with environment variables:", additionalEnvVars)

	// Make a slice for containers for the pods in the deployment.
	containers := make([]corev1.Container, 0)

	if len(checkImage) == 0 {
		err := errors.New("check image url for container is empty: " + checkImage)
		log.Warnln(err.Error())
		return deployment
	}

	// Make the container for the slice.
	var container corev1.Container
	container = createContainerConfig(checkImage)
	containers = append(containers, container)

	graceSeconds := int64(1)

	// Make and define a pod spec with containers.
	podSpec := corev1.PodSpec{
		Containers:                    containers,
		RestartPolicy:                 corev1.RestartPolicyAlways,
		TerminationGracePeriodSeconds: &graceSeconds,
		ServiceAccountName:            checkServiceAccount,
	}

	// Make labels for pod and deployment.
	labels := make(map[string]string, 0)
	labels[defaultLabelKey] = defaultLabelValueBase + strconv.Itoa(int(now.Unix()))
	labels["source"] = "kuberhealthy"

	// Make and define a pod template spec with a pod spec.
	podTemplateSpec := corev1.PodTemplateSpec{
		Spec: podSpec,
	}
	podTemplateSpec.ObjectMeta.Labels = labels
	podTemplateSpec.ObjectMeta.Name = checkDeploymentName
	podTemplateSpec.ObjectMeta.Namespace = checkNamespace

	// Make a selector object for labels.
	labelSelector := metav1.LabelSelector{
		MatchLabels: labels,
	}

	// Calculate max surge and unavailable [#replicas / 2].
	maxSurge := math.Ceil(float64(checkDeploymentReplicas) / float64(2))
	maxUnavailable := math.Ceil(float64(checkDeploymentReplicas) / float64(2))

	// Make a rolling update strategy and define the deployment strategy with it.
	rollingUpdateSpec := v1.RollingUpdateDeployment{
		MaxUnavailable: &intstr.IntOrString{
			IntVal: int32(maxUnavailable),
			StrVal: strconv.Itoa(int(maxUnavailable)),
		},
		MaxSurge: &intstr.IntOrString{
			IntVal: int32(maxSurge),
			StrVal: strconv.Itoa(int(maxSurge)),
		},
	}
	deployStrategy := v1.DeploymentStrategy{
		Type:          v1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &rollingUpdateSpec,
	}

	// Make a deployment spec.
	replicas := int32(checkDeploymentReplicas)
	deploySpec := v1.DeploymentSpec{
		Strategy:        deployStrategy,
		MinReadySeconds: defaultMinReadySeconds,
		Replicas:        &replicas,
		Selector:        &labelSelector,
		Template:        podTemplateSpec,
	}

	// Define the k8s deployment.
	deployment.ObjectMeta.Name = checkDeploymentName
	deployment.ObjectMeta.Namespace = checkNamespace

	// Add the deployment spec to the deployment.
	deployment.Spec = deploySpec

	return deployment
}

// DeploymentResult represents the results from a createDeployment and updateDeployment calls.
type DeploymentResult struct {
	Deployment *v1.Deployment
	Err        error
}

// createDeployment creates a deployment in the cluster with a given deployment specification.
func createDeployment(deploymentConfig *v1.Deployment) chan DeploymentResult {

	createChan := make(chan DeploymentResult)

	go func() {
		log.Infoln("Creating deployment in cluster with name:", deploymentConfig.Name)

		defer close(createChan)

		result := DeploymentResult{}

		deployment, err := client.AppsV1().Deployments(checkNamespace).Create(deploymentConfig)
		if err != nil {
			log.Infoln("Failed to create a deployment in the cluster:", err)
			result.Err = err
			createChan <- result
			return
		}
		if deployment == nil {
			err = errors.New("got a nil deployment result: ")
			log.Errorln("Failed to create a deployment in the cluster: %w", err)
			result.Err = err
			createChan <- result
			return
		}

		for {
			log.Infoln("Watching for deployment to exist.")

			// Watch that it is up.
			watch, err := client.AppsV1().Deployments(checkNamespace).Watch(metav1.ListOptions{
				Watch:         true,
				FieldSelector: "metadata.name=" + deployment.Name,
				// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
			})
			if err != nil {
				result.Err = err
				createChan <- result
				return
			}
			// If the watch is nil, skip to the next loop and make a new watch object.
			if watch == nil {
				continue
			}

			// There can be 2 events here: Available = True status update from deployment or Context timeout.
			for event := range watch.ResultChan() { // Watch for deployment events.

				d, ok := event.Object.(*v1.Deployment)
				if !ok { // Skip the event if it cannot be casted as a v1.Deployment.
					log.Infoln("Got a watch event for a non-deployment object -- ignoring.")
					continue
				}

				log.Debugln("Received an event watching for deployment changes:", d.Name, "got event", event.Type)

				// Look at the status conditions for the deployment object;
				// we want it to be reporting Available = True.
				if deploymentAvailable(d) {
					result.Deployment = d
					createChan <- result
					return
				}

				// If the context has expired, exit.
				select {
				case <-ctx.Done(): // Watch for a context cancellation.
					log.Errorln("Context expired while waiting for deployment to create.")
					err = cleanUp(ctx) // Clean up the deployment.
					if err != nil {
						result.Err = errors.New("failed to clean up properly: " + err.Error())
					}
					createChan <- result
					return
				default:
				}
			}

			// Stop the watch on each loop because we will create a new one.
			watch.Stop()
		}
	}()

	return createChan
}

// createContainerConfig creates a container resource spec and returns it.
func createContainerConfig(imageURL string) corev1.Container {

	log.Infoln("Creating container using image ["+imageURL+"] with environment variables:", additionalEnvVars)

	// Set up a basic container port [default is 80 for HTTP].
	basicPort := corev1.ContainerPort{
		ContainerPort: checkContainerPort,
	}
	containerPorts := []corev1.ContainerPort{basicPort}

	// Make maps for resources.
	// Make and define a map for requests.
	requests := make(map[corev1.ResourceName]resource.Quantity, 0)
	requests[corev1.ResourceCPU] = *resource.NewMilliQuantity(defaultMillicoreRequest, resource.DecimalSI)
	requests[corev1.ResourceMemory] = *resource.NewQuantity(defaultMemoryRequest, resource.BinarySI)

	// Make and define a map for limits.
	limits := make(map[corev1.ResourceName]resource.Quantity, 0)
	limits[corev1.ResourceCPU] = *resource.NewMilliQuantity(defaultMillicoreLimit, resource.DecimalSI)
	limits[corev1.ResourceMemory] = *resource.NewQuantity(defaultMemoryLimit, resource.BinarySI)

	// Make and define a resource requirement struct.
	resources := corev1.ResourceRequirements{
		Requests: requests,
		Limits:   limits,
	}

	// Make a slice for environment variables.
	// Parse passed in environment variables and define the slice.
	envs := make([]corev1.EnvVar, 0)
	for k, v := range additionalEnvVars {
		ev := corev1.EnvVar{
			Name:  k,
			Value: v,
		}
		envs = append(envs, ev)
	}

	// Make a TCP socket for the probe handler.
	tcpSocket := corev1.TCPSocketAction{
		Port: intstr.IntOrString{
			IntVal: checkContainerPort,
			StrVal: strconv.Itoa(int(checkContainerPort)),
		},
	}

	// Make a handler for the probes.
	handler := corev1.Handler{
		TCPSocket: &tcpSocket,
	}

	// Make liveness and readiness probes.
	// Make the liveness probe here.
	liveProbe := corev1.Probe{
		Handler:             handler,
		InitialDelaySeconds: defaultProbeInitialDelaySeconds,
		TimeoutSeconds:      defaultProbeTimeoutSeconds,
		PeriodSeconds:       defaultProbePeriodSeconds,
		SuccessThreshold:    defaultProbeSuccessThreshold,
		FailureThreshold:    defaultProbeFailureThreshold,
	}

	// Make the readiness probe here.
	readyProbe := corev1.Probe{
		Handler:             handler,
		InitialDelaySeconds: defaultProbeInitialDelaySeconds,
		TimeoutSeconds:      defaultProbeTimeoutSeconds,
		PeriodSeconds:       defaultProbePeriodSeconds,
		SuccessThreshold:    defaultProbeSuccessThreshold,
		FailureThreshold:    defaultProbeFailureThreshold,
	}

	// Create the container.
	c := corev1.Container{
		Name:            defaultCheckContainerName,
		Image:           imageURL,
		ImagePullPolicy: defaultImagePullPolicy,
		Ports:           containerPorts,
		Resources:       resources,
		Env:             envs,
		LivenessProbe:   &liveProbe,
		ReadinessProbe:  &readyProbe,
	}

	return c
}

// deleteDeploymentAndWait deletes the created test deployment
func deleteDeploymentAndWait(ctx context.Context) error {

	deleteChan := make(chan error)

	go func() {
		defer close(deleteChan)

		log.Debugln("Checking if deployment has been deleted.")
		for {

			// Check if we have timed out.
			select {
			case <-ctx.Done():
				deleteChan <- fmt.Errorf("timed out while waiting for deployment to delete")
			default:
				log.Debugln("Delete deployment and wait has not yet timed out.")
			}

			// Wait between checks.
			log.Debugln("Waiting 5 seconds before trying again.")
			time.Sleep(time.Second * 5)

			// Watch that it is gone by listing repeatedly.
			deploymentList, err := client.AppsV1().Deployments(checkNamespace).List(metav1.ListOptions{
				FieldSelector: "metadata.name=" + checkDeploymentName,
				// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
			})
			if err != nil {
				log.Errorln("Error listing deployments:", err.Error())
				continue
			}

			// Check for the deployment in the list.
			var deploymentExists bool
			for _, deploy := range deploymentList.Items {
				// If the deployment exists, try to delete it.
				if deploy.GetName() == checkDeploymentName {
					deploymentExists = true
					err = deleteDeployment()
					if err != nil {
						log.Errorln("Error when running a delete on deployment", checkDeploymentName+":", err.Error())
					}
					break
				}
			}

			// If the deployment was not in the list, then we assume it has been deleted.
			if !deploymentExists {
				deleteChan <- nil
				break
			}
		}

	}()

	// Send a delete on the deployment.
	err := deleteDeployment()
	if err != nil {
		log.Infoln("Could not delete deployment:", checkDeploymentName)
	}

	return <-deleteChan
}

// deleteDeployment issues a foreground delete for the check test deployment name.
func deleteDeployment() error {
	log.Infoln("Attempting to delete deployment in", checkNamespace, "namespace.")
	// Make a delete options object to delete the deployment.
	deletePolicy := metav1.DeletePropagationForeground
	graceSeconds := int64(1)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &graceSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	// Delete the deployment and return the result.
	return client.AppsV1().Deployments(checkNamespace).Delete(checkDeploymentName, &deleteOpts)
}

// cleanUpOrphanedDeployment cleans up deployments created from previous checks.
func cleanUpOrphanedDeployment() error {

	cleanUpChan := make(chan error)

	go func() {
		defer close(cleanUpChan)

		// Watch that it is gone.
		watch, err := client.AppsV1().Deployments(checkNamespace).Watch(metav1.ListOptions{
			Watch:         true,
			FieldSelector: "metadata.name=" + checkDeploymentName,
			// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
		})
		if err != nil {
			log.Infoln("Error creating watch client.")
			cleanUpChan <- err
			return
		}
		defer watch.Stop()

		// Watch for a DELETED event.
		for event := range watch.ResultChan() {
			log.Debugln("Received an event watching for service changes:", event.Type)

			d, ok := event.Object.(*v1.Deployment)
			if !ok {
				log.Infoln("Got a watch event for a non-deployment object -- ignoring.")
				continue
			}

			log.Debugln("Received an event watching for deployment changes:", d.Name, "got event", event.Type)

			// We want an event type of DELETED here.
			if event.Type == watchpkg.Deleted {
				log.Infoln("Received a", event.Type, "while watching for deployment with name ["+d.Name+"] to be deleted")
				cleanUpChan <- nil
				return
			}
		}
	}()

	log.Infoln("Removing previous deployment in", checkNamespace, "namespace.")

	// Make a delete options object to delete the service.
	deletePolicy := metav1.DeletePropagationForeground
	graceSeconds := int64(1)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &graceSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	// Send the delete request.
	err := client.AppsV1().Deployments(checkNamespace).Delete(checkDeploymentName, &deleteOpts)
	if err != nil {
		return errors.New("failed to delete previous deployment: " + err.Error())
	}

	return <-cleanUpChan
}

// findPreviousDeployment lists deployments and checks their names and labels to determine if there should
// be an old deployment belonging to this check that should be deleted.
func findPreviousDeployment() (bool, error) {

	log.Infoln("Attempting to find previously created deployment(s) belonging to this check.")

	deploymentList, err := client.AppsV1().Deployments(checkNamespace).List(metav1.ListOptions{})
	if err != nil {
		log.Infoln("error listing deployments:", err)
		return false, err
	}
	if deploymentList == nil {
		log.Infoln("Received an empty list of deployments:", deploymentList)
		return false, errors.New("received empty list of deployments")
	}
	log.Debugln("Found", len(deploymentList.Items), "deployment(s)")

	if debug { // Print out all the found deployments if debug logging is enabled.
		for _, deployment := range deploymentList.Items {
			log.Debugln(deployment.Name)
		}
	}

	// Iterate through deployments and look for previous deployments.
	for _, deployment := range deploymentList.Items {

		// Check using names.
		// if &deployment.Name == nil {
		// 	continue
		// }
		if deployment.Name == checkDeploymentName {
			log.Infoln("Found an old deployment belonging to this check:", deployment.Name)
			return true, nil
		}

		// Check using labels.
		// for k, v := range deployment.Labels {
		// 	if k == defaultLabelKey && v != defaultLabelValueBase+strconv.Itoa(int(now.Unix())) {
		// 		log.Infoln("Found an old deployment belonging to this check.")
		// 		return true, nil
		// 	}
		// }
	}

	log.Infoln("Did not find any old deployment(s) belonging to this check.")
	return false, nil
}

// updateDeployment performs an update on a deployment with a given deployment configuration.  The DeploymentResult
// channel is notified when the rolling update is complete.
func updateDeployment(deploymentConfig *v1.Deployment) chan DeploymentResult {

	updateChan := make(chan DeploymentResult)

	go func() {
		log.Infoln("Performing rolling-update on deployment", deploymentConfig.Name, "to ["+deploymentConfig.Spec.Template.Spec.Containers[0].Image+"]")

		defer close(updateChan)

		result := DeploymentResult{}

		// Get the names of the current pods and ignore them when checking for a completed rolling-update.
		log.Infoln("Creating a blacklist with the current pods that exist.")
		oldPodNames := getPodNames()

		deployment, err := client.AppsV1().Deployments(checkNamespace).Update(deploymentConfig)
		if err != nil {
			log.Infoln("Failed to update deployment in the cluster:", err)
			result.Err = err
			updateChan <- result
			return
		}

		// Watch that it is up.
		watch, err := client.AppsV1().Deployments(checkNamespace).Watch(metav1.ListOptions{
			Watch:         true,
			FieldSelector: "metadata.name=" + deployment.Name,
			// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
		})
		if err != nil {
			result.Err = err
			updateChan <- result
			return
		}

		// Stop the watch on each loop because we will create a new one.
		defer watch.Stop()

		log.Debugln("Watching for deployment rolling-update to complete.")
		newPodStatuses := make(map[string]bool)
		for {
			// There can be 2 events here, Progressing has status "has successfully progressed." or Context timeout.
			select {
			case event := <-watch.ResultChan():
				// Watch for deployment events.
				d, ok := event.Object.(*v1.Deployment)
				if !ok { // Skip the event if it cannot be casted as a v1.Deployment.
					log.Infoln("Got a watch event for a non-deployment object -- ignoring.")
					continue
				}

				log.Debugln("Received an event watching for deployment changes:", d.Name, "got event", event.Type)

				// Look at the status conditions for the deployment object.
				if rollingUpdateComplete(newPodStatuses, oldPodNames) {
					log.Debugln("Rolling-update is assumed to be completed, sending result to channel.")
					result.Deployment = d
					updateChan <- result
					return
				}
			case <-ctx.Done():
				// If the context has expired, exit.
				log.Errorln("Context expired while waiting for deployment to create.")
				err = cleanUp(ctx)
				if err != nil {
					result.Err = errors.New("failed to clean up properly: " + err.Error())
				}
				updateChan <- result
				return
			}
		}
	}()

	return updateChan
}

// waitForDeploymentToDelete waits for the service to be deleted.
func waitForDeploymentToDelete() chan bool {

	// Make and return a channel while we check that the service is gone in the background.
	deleteChan := make(chan bool, 1)

	go func() {
		defer close(deleteChan)
		for {
			_, err := client.AppsV1().Deployments(checkNamespace).Get(checkDeploymentName, metav1.GetOptions{})
			if err != nil {
				log.Debugln("error from Deployments().Get():", err.Error())
				if strings.Contains(err.Error(), "not found") {
					log.Debugln("Deployment deleted.")
					deleteChan <- true
					return
				}
			}
			time.Sleep(time.Millisecond * 250)
		}
	}()

	return deleteChan
}

// rollingUpdateComplete checks the deployment's container images and their statuses and returns
// a boolean based on whether or not the rolling-update is complete.
func rollingUpdateComplete(statuses map[string]bool, oldPodNames []string) bool {

	// Should be looking at pod and pod names NOT containers.
	podList, err := client.CoreV1().Pods(checkNamespace).List(metav1.ListOptions{
		// FieldSelector: "metadata.name=" + checkDeploymentName,
		LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
	})
	if err != nil {
		log.Errorln("failed to list pods:", err)
		return false
	}

	// Look at each pod and see if the deployment update is complete.
	for _, pod := range podList.Items {
		if containsString(pod.Name, oldPodNames) {
			log.Debugln("Skipping", pod.Name, "because it was found in the blacklist.")
			continue
		}

		// If the container in the pod has the correct image, add it to the status map.
		for _, container := range pod.Spec.Containers {
			if container.Image == checkImageURLB {
				if _, ok := statuses[pod.Name]; !ok {
					statuses[pod.Name] = false
				}
			}
		}

		// Check the pod conditions to see if it has finished the rolling-update.
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				if !statuses[pod.Name] {
					log.Debugln("Setting status for", pod.Name, "to true.")
					statuses[pod.Name] = true
				}
			}
		}
	}

	var count int
	for _, status := range statuses {
		if status {
			count++
		}
	}
	log.Infoln(count, "/", checkDeploymentReplicas, "pods have been rolled.")

	// Only return true if ALL pods are up.
	return count == checkDeploymentReplicas
}

// deploymentAvailable checks the status conditions of the deployment and returns a boolean.
// This will return a true if condition 'Available' = status 'True'.
func deploymentAvailable(deployment *v1.Deployment) bool {
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == v1.DeploymentAvailable && condition.Status == corev1.ConditionTrue {
			log.Infoln("Deployment is reporting", condition.Type, "with", condition.Status+".")
			return true
		}
	}
	return false
}

// getPodNames gets the current list of pod names -- used to reference for a completed rolling update.
func getPodNames() []string {
	names := make([]string, 0)

	// Should be looking at pod and pod names NOT containers.
	podList, err := client.CoreV1().Pods(checkNamespace).List(metav1.ListOptions{
		// FieldSelector: "metadata.name=" + checkDeploymentName,
		LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
	})
	if err != nil {
		log.Errorln("failed to list pods:", err)
		return names
	}
	if podList == nil {
		log.Errorln("could not create a list of pod names due to an empty list of pods:", podList)
		return names
	}

	for _, pod := range podList.Items {
		names = append(names, pod.Name)
	}

	return names
}

// containsString returns a boolean value based on whether or not a slice of strings contains
// a string.
func containsString(s string, list []string) bool {
	for _, str := range list {
		if s == str {
			return true
		}
	}
	return false
}
