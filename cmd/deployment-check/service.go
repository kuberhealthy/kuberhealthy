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
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	watchpkg "k8s.io/apimachinery/pkg/watch"
)

// createServiceConfig creates and configures a k8s service and returns the struct (ready to apply with client).
func createServiceConfig(labels map[string]string) *corev1.Service {

	// Make a k8s service.
	service := &corev1.Service{}

	log.Infoln("Creating service resource for", checkNamespace, "namespace.")

	// Make and define a port for the service.
	ports := make([]corev1.ServicePort, 0)
	basicPort := corev1.ServicePort{
		Port: checkLoadBalancerPort, // Port to hit the load balancer on.
		TargetPort: intstr.IntOrString{ // Port to hit the container on.
			IntVal: checkContainerPort,
			StrVal: strconv.Itoa(int(checkContainerPort)),
		},
		Protocol: corev1.ProtocolTCP,
	}
	ports = append(ports, basicPort)

	// Make a service spec.
	serviceSpec := corev1.ServiceSpec{
		Type:     corev1.ServiceTypeClusterIP,
		Ports:    ports,
		Selector: labels,
	}

	// Define the service.
	service.Spec = serviceSpec
	service.Name = checkServiceName
	service.Namespace = checkNamespace

	return service
}

// ServiceResult represents the results from a createService call.
type ServiceResult struct {
	Service *corev1.Service
	Err     error
}

// createService creates a deployment in the cluster with a given deployment specification.
func createService(serviceConfig *corev1.Service) chan ServiceResult {

	createChan := make(chan ServiceResult)

	go func() {
		log.Infoln("Creating service in cluster with name:", serviceConfig.Name)

		defer close(createChan)

		result := ServiceResult{}

		service, err := client.CoreV1().Services(checkNamespace).Create(serviceConfig)
		if err != nil {
			log.Infoln("Failed to create a service in the cluster:", err)
			result.Err = err
			createChan <- result
			return
		}
		if service == nil {
			err = errors.New("got a nil service result: ")
			log.Errorln("Failed to create a service in the cluster: %w", err)
			result.Err = err
			createChan <- result
		}

		for {
			log.Infoln("Watching for service to exist.")

			// Watch that it is up.
			watch, err := client.CoreV1().Services(checkNamespace).Watch(metav1.ListOptions{
				Watch:         true,
				FieldSelector: "metadata.name=" + service.Name,
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

			// There can be 2 events here: Service ingress has at least 1 hostname endpoint or Context timeout.
			select {
			case event := <-watch.ResultChan():
				log.Debugln("Received an event watching for service changes:", event.Type)

				s, ok := event.Object.(*corev1.Service)
				if !ok { // Skip the event if it cannot be casted as a corev1.Service
					log.Debugln("Got a watch event for a non-service object -- ignoring.")
					continue
				}

				// Look at the length of the ClusterIP.
				if serviceAvailable(s) {
					result.Service = s
					createChan <- result
					return
				}
			case s := <-serviceHasClusterIP():
				log.Debugln("A cluster IP belonging to the created service has been found:")
				result.Service = s
				createChan <- result
				return
			case <-ctx.Done():
				log.Errorln("context expired while waiting for service to create.")
				err = cleanUp()
				if err != nil {
					result.Err = err
				}
				createChan <- result
				return
			}

			watch.Stop()
		}
	}()

	return createChan
}

// deleteService deletes the created test service.
func deleteService() error {

	deleteChan := make(chan error)

	go func() {
		defer close(deleteChan)
		for {

			log.Debugln("Creating watch object to look for delete events on services.")

			// Watch that it is gone.
			watch, err := client.CoreV1().Services(checkNamespace).Watch(metav1.ListOptions{
				Watch:         true,
				FieldSelector: "metadata.name=" + checkServiceName,
				// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
			})
			if err != nil {
				log.Infoln("Error creating watch client:", err)
				deleteChan <- err
				return
			}
			// If the watch is nil, skip to the next loop and make a new watch object.
			if watch == nil {
				continue
			}

			// Watch for a DELETED event.
			select {
			case event := <-watch.ResultChan():
				log.Debugln("Received an event watching for service changes:", event.Type)

				s, ok := event.Object.(*corev1.Service)
				if !ok {
					log.Debugln("Got a watch event for a non-service object -- ignoring.")
					continue
				}

				log.Debugln("Received an event watching for service changes:", s.Name, "got event", event.Type)

				// We want an event type of DELETED here.
				if event.Type == watchpkg.Deleted {
					log.Infoln("Received a", event.Type, "while watching for service with name ["+s.Name+"] to be deleted.")
					deleteChan <- nil
					return
				}
			case done := <-waitForServiceToDelete():
				log.Infoln("Received a complete while waiting for service to delete:", done)
				deleteChan <- nil
				return
			case <-ctx.Done():
				log.Errorln("Context expired while waiting for service to be cleaned up.")
				deleteChan <- nil
				return
			}

			watch.Stop()
		}
	}()

	log.Infoln("Attempting to delete service in", checkNamespace, "namespace.")

	// Make a delete options object to delete the service.
	deletePolicy := metav1.DeletePropagationForeground
	graceSeconds := int64(1)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &graceSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	// Delete the service.
	err := client.CoreV1().Services(checkNamespace).Delete(checkServiceName, &deleteOpts)
	if err != nil {
		log.Infoln("Could not delete service:", checkServiceName)
	}

	return <-deleteChan
}

// cleanUpOrphanedService cleans up services created from previous checks.
func cleanUpOrphanedService() error {

	cleanUpChan := make(chan error)

	go func() {
		defer close(cleanUpChan)

		for {
			// Watch that it is gone.
			watch, err := client.CoreV1().Services(checkNamespace).Watch(metav1.ListOptions{
				Watch:         true,
				FieldSelector: "metadata.name=" + checkServiceName,
				// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
			})
			if err != nil {
				log.Infoln("Error creating watch client.")
				cleanUpChan <- err
				return
			}
			// If the watch is nil, skip to the next loop and make a new watch object.
			if watch == nil {
				continue
			}

			// Watch for a DELETED event.
			select {
			case event := <-watch.ResultChan():
				log.Debugln("Received an event watching for service changes:", event.Type)

				s, ok := event.Object.(*corev1.Service)
				if !ok {
					log.Debugln("Got a watch event for a non-service object -- ignoring.")
					continue
				}

				log.Debugln("Received an event watching for service changes:", s.Name, "got event", event.Type)

				// We want an event type of DELETED here.
				if event.Type == watchpkg.Deleted {
					log.Infoln("Received a", event.Type, "while watching for service with name ["+s.Name+"] to be deleted.")
					cleanUpChan <- nil
					return
				}
			case done := <-waitForServiceToDelete():
				log.Infoln("Received a complete while waiting for service to delete:", done)
				cleanUpChan <- nil
				return
			case <-ctx.Done():
				log.Errorln("Context expired while waiting for service to be cleaned up.")
				cleanUpChan <- nil
				return
			}

			watch.Stop()
		}
	}()

	log.Infoln("Removing previous service in", checkNamespace, "namespace.")

	// Make a delete options object to delete the service.
	deletePolicy := metav1.DeletePropagationForeground
	graceSeconds := int64(1)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &graceSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	// Send the delete request.
	err := client.CoreV1().Services(checkNamespace).Delete(checkServiceName, &deleteOpts)
	if err != nil {
		return errors.New("failed to delete previous service: " + err.Error())
	}

	return <-cleanUpChan
}

// findPreviousService lists services and checks their names and labels to determine if there should
// be an old service belonging to this check that should be deleted.
func findPreviousService() (bool, error) {

	log.Infoln("Attempting to find previously created service(s) belonging to this check.")

	serviceList, err := client.CoreV1().Services(checkNamespace).List(metav1.ListOptions{})
	if err != nil {
		log.Infoln("Error listing services:", err)
		return false, err
	}
	if serviceList == nil {
		log.Infoln("Received an empty list of services:", serviceList)
		return false, errors.New("received empty list of services")
	}

	log.Debugln("Found", len(serviceList.Items), "service(s).")

	if debug { // Print out all the found deployments if debug logging is enabled.
		for _, svc := range serviceList.Items {
			log.Debugln(svc.Name)
		}
	}

	// Iterate through services and look for previous services.
	for _, svc := range serviceList.Items {

		// Check using names.
		if &svc.Name == nil {
			continue
		}
		if svc.Name == checkServiceName {
			log.Infoln("Found an old service belonging to this check:", svc.Name)
			return true, nil
		}

		// Check using labels
		// labels := svc.Labels
		// for k, v := range labels {
		// 	if k == defaultLabelKey && v != defaultLabelValueBase+strconv.Itoa(int(now.Unix())) {
		// 		log.Infoln("Found an old service belonging to this check.")
		// 		return true, nil
		// 	}
		// }
	}

	log.Infoln("Did not find any old service(s) belonging to this check.")
	return false, nil
}

// getServiceLoadBalancerHostname retrieves the hostname for the load balancer utilized for the service.
func getServiceLoadBalancerHostname() string {

	svc, err := client.CoreV1().Services(checkNamespace).Get(checkServiceName, metav1.GetOptions{})
	if err != nil {
		log.Infoln("Error occurred attempting to list service while retrieving service hostname:", err)
		return ""
	}

	log.Debugln("Retrieving a load balancer hostname belonging to:", svc.Name)
	if len(svc.Status.LoadBalancer.Ingress) != 0 {
		log.Infoln("Found service load balancer ingress hostname:", svc.Status.LoadBalancer.Ingress[0].Hostname)
		return svc.Status.LoadBalancer.Ingress[0].Hostname
	}
	return ""
}

// getServiceClusterIP retrieves the cluster IP address utilized for the service
func getServiceClusterIP() string {

	svc, err := client.CoreV1().Services(checkNamespace).Get(checkServiceName, metav1.GetOptions{})
	if err != nil {
		log.Errorln("Error occurred attempting to list service while retrieving service cluster IP:", err)
		return ""
	}
	if svc == nil {
		log.Errorln("Failed to get service, received a nil object:", svc)
		return ""
	}

	log.Debugln("Retrieving a cluster IP belonging to:", svc.Name)
	if len(svc.Spec.ClusterIP) != 0 {
		log.Infoln("Found service cluster IP address:", svc.Spec.ClusterIP)
		return svc.Spec.ClusterIP
	}
	return ""
}

// waitForServiceToDelete waits for the service to be deleted.
func waitForServiceToDelete() chan bool {

	// Make and return a channel while we check that the service is gone in the background.
	deleteChan := make(chan bool)

	go func() {
		defer close(deleteChan)
		for {
			_, err := client.CoreV1().Services(checkNamespace).Get(checkServiceName, metav1.GetOptions{})
			if err != nil {
				log.Debugln("error from Services().Get():", err.Error())
				if strings.Contains(err.Error(), "not found") {
					log.Debugln("Service deleted.")
					deleteChan <- true
					return
				}
			}
			time.Sleep(time.Millisecond * 250)
		}
	}()

	return deleteChan
}

// serviceAvailable checks the amount of ingress endpoints associated to the service.
// This will return a true if there is at least 1 hostname endpoint.
func serviceAvailable(service *corev1.Service) bool {
	if service == nil {
		return false
	}
	if len(service.Spec.ClusterIP) != 0 {
		log.Infoln("Cluster IP found:", service.Spec.ClusterIP)
		return true
	}
	// if len(service.Status.LoadBalancer.Ingress) != 0 {
	// 	log.Infoln("Service ingress hostname found:", service.Status.LoadBalancer.Ingress[0].Hostname)
	// 	return true
	// }
	return false
}

// serviceHasClusterIP checks the service object to see if a cluster IP has been
// allocated to it yet and returns when a cluster IP exists.
func serviceHasClusterIP() chan *corev1.Service {

	resultChan := make(chan *corev1.Service)

	go func() {
		defer close(resultChan)

		for {
			svc, err := client.CoreV1().Services(checkNamespace).Get(checkServiceName, metav1.GetOptions{})
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			if len(svc.Spec.ClusterIP) != 0 {
				resultChan <- svc
				return
			}
		}
	}()

	return resultChan
}
