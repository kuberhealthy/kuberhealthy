package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	service.Name = checkServiceName //+ "-" + strconv.Itoa(int(now.Unix()))
	service.Namespace = checkNamespace

	return service
}

// ServiceResult represents the results from a createService call.
type ServiceResult struct {
	Service *corev1.Service
	Err     error
}

// createService creates a deployment in the cluster with a given deployment specification.
func createService(ctx context.Context, serviceConfig *corev1.Service) chan ServiceResult {

	createChan := make(chan ServiceResult)

	go func() {
		log.Infoln("Creating service in cluster with name:", serviceConfig.Name)

		defer close(createChan)

		result := ServiceResult{}

		service, err := client.CoreV1().Services(checkNamespace).Create(ctx, serviceConfig, metav1.CreateOptions{})
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
			return
		}

		for {
			log.Infoln("Watching for service to exist.")

			// Watch that it is up.
			watch, err := client.CoreV1().Services(checkNamespace).Watch(ctx, metav1.ListOptions{
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
			case s := <-serviceHasClusterIP(ctx):
				log.Debugln("A cluster IP belonging to the created service has been found:")
				result.Service = s
				createChan <- result
				return
			case <-ctx.Done():
				log.Errorln("context expired while waiting for service to create.")
				err = cleanUp(ctx)
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

// deleteServiceAndWait deletes the created test service.
func deleteServiceAndWait(ctx context.Context) error {

	deleteChan := make(chan error)

	// TODO - pass in a contet to abort watches?
	go func() {
		defer close(deleteChan)

		log.Debugln("Checking if service has been deleted.")
		for {

			// Check if we have timed out.
			select {
			case <-ctx.Done():
				deleteChan <- fmt.Errorf("timed out while waiting for service to delete")
				return
			default:
				log.Debugln("Delete service and wait has not yet timed out.")
			}

			// Wait between checks.
			log.Debugln("Waiting 5 seconds before trying again.")
			time.Sleep(time.Second * 5)

			// Watch that it is gone by listing repeatedly.
			serviceList, err := client.CoreV1().Services(checkNamespace).List(ctx, metav1.ListOptions{
				FieldSelector: "metadata.name=" + checkServiceName,
				// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
			})
			if err != nil {
				log.Errorln("Error creating service listing client:", err.Error())
				continue
			}

			// Check for the service in the list.
			var serviceExists bool
			for _, svc := range serviceList.Items {
				// If the service exists, try to delete it.
				if svc.GetName() == checkServiceName {
					serviceExists = true
					err = deleteService(ctx)
					if err != nil {
						log.Errorln("Error when running a delete on service", checkServiceName+":", err.Error())
					}
					break
				}
			}

			// If the service was not in the list, then we assume it has been deleted.
			if !serviceExists {
				deleteChan <- nil
				break
			}
		}

	}()

	// Send a delete on the service.
	err := deleteService(ctx)
	if err != nil {
		log.Infoln("Could not delete service:", checkServiceName)
	}

	return <-deleteChan
}

// deleteService issues a foreground delete for the check test service name.
func deleteService(ctx context.Context) error {
	log.Infoln("Attempting to delete service", checkServiceName, "in", checkNamespace, "namespace.")
	// Make a delete options object to delete the service.
	deletePolicy := metav1.DeletePropagationForeground
	graceSeconds := int64(1)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &graceSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	// Delete the service and return the result.
	return client.CoreV1().Services(checkNamespace).Delete(ctx, checkServiceName, deleteOpts)
}

// findPreviousService lists services and checks their names and labels to determine if there should
// be an old service belonging to this check that should be deleted.
func findPreviousService(ctx context.Context) (bool, error) {

	log.Infoln("Attempting to find previously created service(s) belonging to this check.")

	serviceList, err := client.CoreV1().Services(checkNamespace).List(ctx, metav1.ListOptions{})
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
			log.Debugln("Service:", svc.Name)
		}
	}

	// Iterate through services and look for previous services.
	for _, svc := range serviceList.Items {

		// Check using names.
		// if &svc.Name == nil {
		// 	continue
		// }
		if svc.Name == checkServiceName {
			log.Infoln("Found an old service belonging to this check:", svc.Name)
			return true, nil
		}

		// Check using labels.
		// for k, v := range svc.Labels {
		// 	if k == defaultLabelKey && v != defaultLabelValueBase+strconv.Itoa(int(now.Unix())) {
		// 		log.Infoln("Found an old service belonging to this check.")
		// 		return true, nil
		// 	}
		// }
	}

	log.Infoln("Did not find any old service(s) belonging to this check.")
	return false, nil
}

// getServiceClusterIP retrieves the cluster IP address utilized for the service
func getServiceClusterIP(ctx context.Context) string {

	svc, err := client.CoreV1().Services(checkNamespace).Get(ctx, checkServiceName, metav1.GetOptions{})
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
	return false
}

// serviceHasClusterIP checks the service object to see if a cluster IP has been
// allocated to it yet and returns when a cluster IP exists.
func serviceHasClusterIP(ctx context.Context) chan *corev1.Service {

	resultChan := make(chan *corev1.Service)

	go func() {
		defer close(resultChan)

		for {
			svc, err := client.CoreV1().Services(checkNamespace).Get(ctx, checkServiceName, metav1.GetOptions{})
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
