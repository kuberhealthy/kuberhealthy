package main

import (
	"errors"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
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
	}
	ports = append(ports, basicPort)

	// Make a service spec.
	serviceSpec := corev1.ServiceSpec{
		Type:     corev1.ServiceTypeLoadBalancer,
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

			// There can be 2 events here: Service ingress has at least 1 hostname endpoint or Context timeout.
			for event := range watch.ResultChan() {
				log.Debugln("Received an event watching for service changes:", event.Type)

				s, ok := event.Object.(*corev1.Service)
				if !ok { // Skip the event if it cannot be casted as a corev1.Service.
					log.Debugln("Got a watch event for a non-service object -- ignoring.")
					continue
				}

				// Look at the length of the ingress list;
				// we want there to be at least 1 hostname endpoint.
				if serviceAvailable(s) {
					result.Service = s
					createChan <- result
					return
				}

				select {
				case <-ctx.Done(): // Watch for a context cancellation.
					log.Errorln("Context expired while waiting for service to create.")
					err = cleanUp() // Clean up the service and deployment.
					if err != nil {
						result.Err = err
					}
					createChan <- result
					return
				default:
				}
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

			// Watch for a DELETED event.
			for event := range watch.ResultChan() {

				log.Debugln("Received an event watching for service changes:", event.Type)

				s, ok := event.Object.(*corev1.Service)
				if !ok {
					log.Debugln("Got a watch event for a non-service object -- ignoring.")
					continue
				}

				// We want an event type of DELETED here.
				if event.Type == "DELETED" {
					log.Infoln("Received a", event.Type, "while watching for service with name ["+s.Name+"] to be deleted.")
					deleteChan <- nil
					return
				}
			}

			watch.Stop()
		}
	}()

	log.Infoln("Attempting to delete service in", checkNamespace, "namespace.")
	if client == nil {
		err := errors.New("nil kubernetes client")
		log.Debugln("deleteService:", err)
		return err
	}

	// Make a delete options object to delete the service.
	deletePolicy := metav1.DeletePropagationForeground
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: aws.Int64(1),
		PropagationPolicy:  &deletePolicy,
	}

	// Delete the service.
	err := client.CoreV1().Services(checkNamespace).Delete(checkServiceName, &deleteOpts)
	if err != nil {
		log.Infoln("Could not delete service:", checkServiceName)
		log.Infoln("Beginning backoff retry loop to delete service.")
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

			// Watch for a DELETED event.
			select {
			case event := <-watch.ResultChan():
				log.Debugln("Received an event watching for service changes:", event.Type)

				s, ok := event.Object.(*corev1.Service)
				if !ok {
					log.Debugln("Got a watch event for a non-service object -- ignoring.")
					continue
				}

				// We want an event type of DELETED here.
				if event.Type == "DELETED" {
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
	if client == nil {
		return errors.New("nil kubernetes client")
	}

	// Make a delete options object to delete the service.
	deletePolicy := metav1.DeletePropagationForeground
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: aws.Int64(1),
		PropagationPolicy:  &deletePolicy,
	}

	// Hammer the API with a couple of delete requests.
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
	log.Debugln("Found", len(serviceList.Items), "service(s).")

	if debug { // Print out all the found deployments if debug logging is enabled.
		for _, svc := range serviceList.Items {
			log.Debugln(svc.Name)
		}
	}

	// Iterate through services and look for previous services.
	for _, svc := range serviceList.Items {

		// Check using names.
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

// waitForServiceToDelete waits for the service to be deleted.
func waitForServiceToDelete() chan bool {

	// Make and return a channel while we check that the service is gone in the background.
	deleteChan := make(chan bool)

	go func() {
		defer close(deleteChan)
		for {
			_, err := client.CoreV1().Services(checkNamespace).Get(checkServiceName, metav1.GetOptions{})
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					deleteChan <- true
					return
				}
			}
		}
	}()

	return deleteChan
}

// serviceAvailable checks the amount of ingress endpoints associated to the service.
// This will return a true if there is at least 1 hostname endpoint.
func serviceAvailable(service *corev1.Service) bool {
	if len(service.Status.LoadBalancer.Ingress) != 0 {
		log.Infoln("Service ingress hostname found:", service.Status.LoadBalancer.Ingress[0].Hostname)
		return true
	}
	return false
}
