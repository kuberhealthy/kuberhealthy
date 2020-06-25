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
	"os"
	"strconv"
	"strings"
	"time"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"

	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watchpkg "k8s.io/apimachinery/pkg/watch"
)

const (
	// Default storage values.
	defaultLabelKey        = "storage-timestamp"
	defaultLabelValueBase  = "unix-"
	defaultMinReadySeconds = 5

	// Default container values.
	defaultImagePullPolicy = corev1.PullIfNotPresent

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

// createStorageConfig creates and configures a k8s PVC and returns the struct (ready to apply with client).
func createStorageConfig(pvcname string) *corev1.PersistentVolumeClaim {

	// Make a k8s pvc.
	// https://kubernetes.io/docs/concepts/storage/persistent-volumes/
	pvc := &corev1.PersistentVolumeClaim{}

	log.Infof("*****Creating persistent volume claim resource with %s in %s namespace environment variables: %+v", pvcname, checkNamespace, additionalEnvVars)

	// Make labels for PVC.
	labels := make(map[string]string, 0)
	labels[defaultLabelKey] = defaultLabelValueBase + strconv.Itoa(int(now.Unix()))
	labels["source"] = "kuberhealthy"

	// Make a pvc spec.
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
		//Selector: ""
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse(pvcSize),
			},
		},
	}
	// By default, don't specify a storage class name and let it default to the cluster default
	// eg: kubectl get storageclasses and see which entry has (default) next to it
	// but if we did specify the storage class add it in now
	storageClassNameEnv = os.Getenv("CHECK_STORAGE_PVC_STORAGE_CLASS_NAME")
	if storageClassNameEnv != "" {
		log.Infoln("Changing from default PVC storage class to", storageClassNameEnv)
		pvcSpec.StorageClassName = &storageClassNameEnv
	}

	// Define the k8s storage.
	pvc.Name = pvcname
	pvc.Namespace = checkNamespace
	pvc.Labels = labels

	// Add the storage spec to the storage.
	pvc.Spec = pvcSpec

	log.Infoln("PVC ", pvcname, " is", pvc, "namespace environment variables:chris", additionalEnvVars)
	return pvc
}

// initializeStorageConfig creates and configures a k8s pod to initialize storage at PVC and returns the struct (ready to apply with client).
func initializeStorageConfig(jobName string, pvcName string) *batchv1.Job {

	// Make a Job/Pod
	job := &batchv1.Job{}

	log.Infof("Creating a job %s in %s namespace environment variables: %v", jobName, checkNamespace, additionalEnvVars)

	// Make labels for Pod
	labels := make(map[string]string, 0)
	labels[defaultLabelKey] = defaultLabelValueBase + strconv.Itoa(int(now.Unix()))
	labels["source"] = "kuberhealthy"

	// Make a Pod spec.
	var command = []string{defaultCheckStorageInitCommand}
	var args = []string{"-c", defaultCheckStorageInitCommandArgs}
	pvc := &corev1.PersistentVolumeClaimVolumeSource{
		ClaimName: pvcName,
	}
	jobSpec := batchv1.JobSpec{
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: jobName,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:            jobName,
						Image:           checkStorageInitImage,
						ImagePullPolicy: defaultImagePullPolicy,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "data",
							MountPath: "/data",
						}},
						Command: command,
						Args:    args,
					},
				},
				RestartPolicy: v1.RestartPolicyNever,
				Volumes: []corev1.Volume{{
					Name:         "data",
					VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: pvc},
				}},
			},
		},
	}

	// Define the k8s storage.
	job.Name = jobName
	job.Namespace = checkNamespace
	job.Labels = labels

	// Add the storage spec to the storage.
	job.Spec = jobSpec

	log.Infoln("Job ", jobName, " is", job, "namespace environment variables:", additionalEnvVars)
	return job
}

// checkNodeConfig creates and configures a k8s job to initialize storage at PVC and returns the struct (ready to apply with client).
func checkNodeConfig(jobName string, pvcName string, node string) *batchv1.Job {

	// Make a Job
	job := &batchv1.Job{}

	log.Infoln("Creating a job", jobName, " in", checkNamespace, "namespace environment variables:", additionalEnvVars)

	// Make labels for Job
	labels := make(map[string]string, 0)
	labels[defaultLabelKey] = defaultLabelValueBase + strconv.Itoa(int(now.Unix()))
	labels["source"] = "kuberhealthy"

	// Make a Job spec.
	var command = []string{defaultCheckStorageCommand}
	var args = []string{"-c", defaultCheckStorageCommandArgs}
	pvc := &corev1.PersistentVolumeClaimVolumeSource{
		ClaimName: pvcName,
	}
	jobSpec := batchv1.JobSpec{
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: jobName,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:            jobName,
						Image:           checkStorageImage,
						ImagePullPolicy: defaultImagePullPolicy,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "data",
							MountPath: "/data",
						}},
						Command: command,
						Args:    args,
					},
				},
				NodeName:      node,
				RestartPolicy: v1.RestartPolicyNever,
				Volumes: []corev1.Volume{{
					Name:         "data",
					VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: pvc},
				}},
			},
		},
	}

	// Define the k8s storage.
	job.Name = jobName
	job.Namespace = checkNamespace
	job.Labels = labels

	// Add the storage spec to the storage.
	job.Spec = jobSpec

	log.Infoln("Job ", jobName, " is", job, "namespace environment variables:", additionalEnvVars)
	return job
}

//TODO do we really need different storage result structs?
// InitStorageResult represents the results from a createInitStorage.
type InitStorageResult struct {
	Pod *batchv1.Job
	Err error
}

// CheckStorageResult represents the results from a createStorageConfig
type CheckStorageResult struct {
	Pod *batchv1.Job
	Err error
}

// StorageResult represents the results from a createStorage.
type StorageResult struct {
	PersistentVolumeClaim *corev1.PersistentVolumeClaim
	Err                   error
}

// createStorage creates a storage in the cluster with a given storage specification.
func createStorage(pvcConfig *corev1.PersistentVolumeClaim) chan StorageResult {

	createChan := make(chan StorageResult)

	go func() {
		log.Infoln("Creating storage in cluster with name:", pvcConfig.Name)

		defer close(createChan)

		result := StorageResult{}

		storage, err := client.CoreV1().PersistentVolumeClaims(checkNamespace).Create(pvcConfig)
		if err != nil {
			log.Infoln("Failed to create a storage in the cluster:", err)
			result.Err = err
			createChan <- result
			return
		}
		if storage == nil {
			err = errors.New("got a nil storage result: ")
			log.Errorln("Failed to create a storage in the cluster: %w", err)
			result.Err = err
			createChan <- result
			return
		}

		for {
			log.Infoln("Watching for storage to exist.")

			// Watch that it is up.
			watch, err := client.CoreV1().PersistentVolumeClaims(checkNamespace).Watch(metav1.ListOptions{
				Watch:         true,
				FieldSelector: "metadata.name=" + storage.Name,
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

			// There can be 2 events here: Available = True status update from storage or Context timeout.
			for event := range watch.ResultChan() { // Watch for storage events.

				d, ok := event.Object.(*corev1.PersistentVolumeClaim)
				if !ok { // Skip the event if it cannot be casted as a corev1.PVC.
					log.Infoln("Got a watch event for a non-storage object -- ignoring.")
					continue
				}

				log.Debugln("Received an event watching for storage changes:", d.Name, "got event", event.Type)

				// Look at the status conditions for the storage object;
				// we want it to be reporting Available = True.
				if storageAvailable(d) {
					result.PersistentVolumeClaim = d
					createChan <- result
					return
				}

				// If the context has expired, exit.
				select {
				case <-ctx.Done(): // Watch for a context cancellation.
					log.Errorln("Context expired while waiting for storage to create.")
					err = cleanUp(ctx) // Clean up the storage.
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

// initializeStorage initializes the PVC with known data using a Job
func initializeStorage(job *batchv1.Job) chan InitStorageResult {

	createChan := make(chan InitStorageResult)

	go func() {
		log.Infoln("Initializing storage in cluster with name:", job.Name)

		defer close(createChan)

		result := InitStorageResult{}

		initStorage, err := client.BatchV1().Jobs(checkNamespace).Create(job)
		if err != nil {
			log.Infoln("Failed to create a storage initializer Job in the cluster:", err)
			result.Err = err
			createChan <- result
			return
		}
		if initStorage == nil {
			err = errors.New("got a nil storage initializer Job result: ")
			log.Errorln("Failed to create a storage initializer Job in the cluster: %w", err)
			result.Err = err
			createChan <- result
			return
		}

		for {
			log.Infoln("Watching for storage initializer Job to exist.")

			// Watch that it is up.
			watch, err := client.BatchV1().Jobs(checkNamespace).Watch(metav1.ListOptions{
				Watch:         true,
				FieldSelector: "metadata.name=" + initStorage.Name,
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

			// There can be 2 events here: Available = True status update from storage or Context timeout.
			for event := range watch.ResultChan() { // Watch for storage events.

				// TODO can we move this into a more generic coordinator so that we're waiting for the entire check to spin up instead of this mess?
				d, ok := event.Object.(*batchv1.Job)
				if !ok { // Skip the event if it cannot be casted as a batachv1.Job.
					log.Infoln("Got a watch event for a non-storage object -- ignoring.")
					continue
				}

				log.Debugln("Received an event watching for storage changes:", d.Name, "got event", event.Type)

				// Look at the status conditions for the storage object;
				// we want it to be reporting Available = True.
				if storageInitialized(d) {
					result.Pod = d
					createChan <- result
					return
				}

				// If the context has expired, exit.
				select {
				case <-ctx.Done(): // Watch for a context cancellation.
					log.Errorln("Context expired while waiting for storage to create.")
					err = cleanUp(ctx) // Clean up the storage.
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

// checkStorage checks the node the PVC with known data using a Job
func checkStorage(job *batchv1.Job) chan CheckStorageResult {

	createChan := make(chan CheckStorageResult)

	go func() {
		log.Infoln("Initializing storage check in cluster with name:", job.Name)

		defer close(createChan)

		result := CheckStorageResult{}

		checkStorage, err := client.BatchV1().Jobs(checkNamespace).Create(job)
		if err != nil {
			log.Infoln("Failed to create a storage check Job in the cluster:", err)
			result.Err = err
			createChan <- result
			return
		}
		if checkStorage == nil {
			err = errors.New("got a nil storage check Job result: ")
			log.Errorln("Failed to create a storage check Job in the cluster: %w", err)
			result.Err = err
			createChan <- result
			return
		}

		for {
			log.Infoln("Watching for storage check Job to exist.")

			// Watch that it is up.
			watch, err := client.BatchV1().Jobs(checkNamespace).Watch(metav1.ListOptions{
				Watch:         true,
				FieldSelector: "metadata.name=" + checkStorage.Name,
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

			// There can be 2 events here: Available = True status update from storage or Context timeout.
			for event := range watch.ResultChan() { // Watch for storage events.

				// TODO can we move this into a more generic coordinator so that we're waiting for the entire check to spin up instead of this mess?
				d, ok := event.Object.(*batchv1.Job)
				if !ok { // Skip the event if it cannot be casted as a batch1.Job.
					log.Infoln("Got a watch event for a non-storage object -- ignoring.")
					continue
				}

				log.Debugln("Received an event watching for storage changes:", d.Name, "got event", event.Type)

				// Look at the status conditions for the storage object;
				// we want it to be reporting Available = True.
				if storageInitialized(d) {
					result.Pod = d
					createChan <- result
					return
				}

				// If the context has expired, exit.
				select {
				case <-ctx.Done(): // Watch for a context cancellation.
					log.Errorln("Context expired while waiting for storage to create.")
					err = cleanUp(ctx) // Clean up the storage.
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

// deleteStorageAndWait deletes the created test storage
func deleteStorageAndWait(ctx context.Context) error {

	deleteChan := make(chan error)

	go func() {
		defer close(deleteChan)

		log.Debugln("Checking if storage has been deleted.")
		for {

			// Check if we have timed out.
			select {
			case <-ctx.Done():
				deleteChan <- fmt.Errorf("timed out while waiting for storage to delete")
			default:
				log.Debugln("Delete storage and wait has not yet timed out.")
			}

			// Wait between checks.
			log.Debugln("Waiting 5 seconds before trying again.")
			time.Sleep(time.Second * 5)

			// Watch that it is gone by listing repeatedly.
			storageList, err := client.CoreV1().PersistentVolumeClaims(checkNamespace).List(metav1.ListOptions{
				FieldSelector: "metadata.name=" + checkStorageName,
			})
			if err != nil {
				log.Errorln("Error listing storages:", err.Error())
				continue
			}

			// Check for the storage in the list.
			var storageExists bool
			for _, deploy := range storageList.Items {
				// If the storage exists, try to delete it.
				if deploy.GetName() == checkStorageName {
					storageExists = true
					err = deleteStorage()
					if err != nil {
						log.Errorln("Error when running a delete on storage", checkStorageName+":", err.Error())
					}
					break
				}
			}

			// If the storage was not in the list, then we assume it has been deleted.
			if !storageExists {
				deleteChan <- nil
				break
			}
		}

	}()

	// Send a delete on the storage.
	err := deleteStorage()
	if err != nil {
		log.Infoln("Could not delete storage:", checkStorageName)
	}

	return <-deleteChan
}

// deleteStorageCheckAndWait deletes the created storage check job
func deleteStorageCheckAndWait(ctx context.Context) error {

	deleteChan := make(chan error)
	//TODO Hardcoded silly that should be abstracted and put upstream
	jobName := checkStorageName + "-check-job"

	go func() {
		defer close(deleteChan)

		log.Debugln("Checking if storage check job has been deleted.")
		for {

			// Check if we have timed out.
			select {
			case <-ctx.Done():
				deleteChan <- fmt.Errorf("timed out while waiting for storage check job to delete")
			default:
				log.Debugln("Delete storage check job and wait has not yet timed out.")
			}

			// Wait between checks.
			log.Debugln("Waiting 5 seconds before trying again.")
			time.Sleep(time.Second * 5)

			// Watch that it is gone by listing repeatedly.
			jobList, err := client.BatchV1().Jobs(checkNamespace).List(metav1.ListOptions{
				FieldSelector: "metadata.name=" + jobName,
			})
			if err != nil {
				log.Errorln("Error listing Jobs:", err.Error())
				continue
			}

			// Check for the Job in the list.
			var jobExists bool
			for _, deploy := range jobList.Items {
				// If the storage init job exists, try to delete it.
				if deploy.GetName() == jobName {
					jobExists = true
					err = deleteStorageCheckJob(jobName)
					if err != nil {
						log.Errorln("Error when running a delete on storage init job", jobName+":", err.Error())
					}
					break
				}
			}

			// If the storage init job was not in the list, then we assume it has been deleted.
			if !jobExists {
				deleteChan <- nil
				break
			}
		}

	}()

	// Send a delete on the storage.
	err := deleteStorageCheckJob(jobName)
	if err != nil {
		log.Infoln("Could not delete storage init job :", checkStorageName)
	}

	return <-deleteChan
}

// deleteStorageInitJobAndWait deletes the created job to initialize storage
func deleteStorageInitJobAndWait(ctx context.Context) error {

	deleteChan := make(chan error)
	//TODO Hardcoded silly that should be abstracted and put upstream
	jobName := checkStorageName + "-init-job"

	go func() {
		defer close(deleteChan)

		log.Debugln("Checking if storage init job has been deleted.")
		for {

			// Check if we have timed out.
			select {
			case <-ctx.Done():
				deleteChan <- fmt.Errorf("timed out while waiting for storage init job to delete")
			default:
				log.Debugln("Delete storage init job and wait has not yet timed out.")
			}

			// Wait between checks.
			log.Debugln("Waiting 5 seconds before trying again.")
			time.Sleep(time.Second * 5)

			// Watch that it is gone by listing repeatedly.
			jobList, err := client.BatchV1().Jobs(checkNamespace).List(metav1.ListOptions{
				FieldSelector: "metadata.name=" + jobName,
			})
			if err != nil {
				log.Errorln("Error listing Jobs:", err.Error())
				continue
			}

			// Check for the Job in the list.
			var jobExists bool
			for _, deploy := range jobList.Items {
				// If the storage init job exists, try to delete it.
				if deploy.GetName() == checkStorageName {
					jobExists = true
					err = deleteStorageInitJob(jobName)
					if err != nil {
						log.Errorln("Error when running a delete on storage init job", checkStorageName+":", err.Error())
					}
					break
				}
			}

			// If the storage init job was not in the list, then we assume it has been deleted.
			if !jobExists {
				deleteChan <- nil
				break
			}
		}

	}()

	// Send a delete on the storage.
	err := deleteStorageInitJob(jobName)
	if err != nil {
		log.Infoln("Could not delete storage init job :", checkStorageName)
	}

	return <-deleteChan
}

// deleteStorage issues a foreground delete for the check test storage name.
func deleteStorage() error {
	log.Infoln("Attempting to delete storage in", checkNamespace, "namespace.")
	// Make a delete options object to delete the storage.
	deletePolicy := metav1.DeletePropagationForeground
	graceSeconds := int64(1)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &graceSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	// Delete the storage and return the result.
	return client.CoreV1().PersistentVolumeClaims(checkNamespace).Delete(checkStorageName, &deleteOpts)
}

// deleteStorageInitJob issues a foreground delete for the test storage init name.
func deleteStorageInitJob(job string) error {
	log.Infoln("Attempting to delete storage init job ", job, " in", checkNamespace, "namespace.")
	// Make a delete options object to delete the storage.
	deletePolicy := metav1.DeletePropagationForeground
	graceSeconds := int64(1)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &graceSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	// Delete the storage and return the result.
	return client.BatchV1().Jobs(checkNamespace).Delete(job, &deleteOpts)
}

// deleteStorageCheckJob issues a foreground delete for the storage check job name.
func deleteStorageCheckJob(job string) error {
	log.Infoln("Attempting to delete storage check job in", checkNamespace, "namespace.")
	// Make a delete options object to delete the storage.
	deletePolicy := metav1.DeletePropagationForeground
	graceSeconds := int64(1)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &graceSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	// Delete the storage and return the result.
	return client.BatchV1().Jobs(checkNamespace).Delete(job, &deleteOpts)
}

// cleanUpOrphanedStorage cleans up storages created from previous checks.
func cleanUpOrphanedStorage() error {

	cleanUpChan := make(chan error)

	go func() {
		defer close(cleanUpChan)

		// Watch that it is gone.
		watch, err := client.CoreV1().PersistentVolumeClaims(checkNamespace).Watch(metav1.ListOptions{
			Watch:         true,
			FieldSelector: "metadata.name=" + checkStorageName,
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

			d, ok := event.Object.(*corev1.PersistentVolumeClaim)
			if !ok {
				log.Infoln("Got a watch event for a non-storage object -- ignoring.")
				continue
			}

			log.Debugln("Received an event watching for storage changes:", d.Name, "got event", event.Type)

			// We want an event type of DELETED here.
			if event.Type == watchpkg.Deleted {
				log.Infoln("Received a", event.Type, "while watching for storage with name ["+d.Name+"] to be deleted")
				cleanUpChan <- nil
				return
			}
		}
	}()

	log.Infoln("Removing previous storage in", checkNamespace, "namespace.")

	// Make a delete options object to delete the service.
	deletePolicy := metav1.DeletePropagationForeground
	graceSeconds := int64(1)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &graceSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	// Send the delete request.
	err := client.CoreV1().PersistentVolumeClaims(checkNamespace).Delete(checkStorageName, &deleteOpts)
	if err != nil {
		return errors.New("failed to delete previous storage: " + err.Error())
	}

	return <-cleanUpChan
}

// findPreviousStorage lists storages and checks their names and labels to determine if there should
// be an old storage belonging to this check that should be deleted.
func findPreviousStorage() (bool, error) {

	log.Infoln("Attempting to find previously created storage(s) belonging to this check.")

	storageList, err := client.CoreV1().PersistentVolumeClaims(checkNamespace).List(metav1.ListOptions{})
	if err != nil {
		log.Infoln("error listing storages:", err)
		return false, err
	}
	if storageList == nil {
		log.Infoln("Received an empty list of storages:", storageList)
		return false, errors.New("received empty list of storages")
	}
	log.Debugln("Found", len(storageList.Items), "storage(s)")

	if debug { // Print out all the found storages if debug logging is enabled.
		for _, storage := range storageList.Items {
			log.Debugln(storage.Name)
		}
	}

	// Iterate through storages and look for previous storages.
	for _, storage := range storageList.Items {

		if storage.Name == checkStorageName {
			log.Infoln("Found an old storage belonging to this check:", storage.Name)
			return true, nil
		}
	}

	log.Infoln("Did not find any old storage(s) belonging to this check.")
	return false, nil
}

// findPreviousStorageInitJob lists Jobs and checks their names and labels to determine if there should
// be an old storage init job belonging to this check that should be deleted.
func findPreviousStorageInitJob() (bool, error) {

	log.Infoln("Attempting to find previously created storage init job belonging to this check.")

	jobList, err := client.BatchV1().Jobs(checkNamespace).List(metav1.ListOptions{})
	if err != nil {
		log.Infoln("error listing Jobs:", err)
		return false, err
	}
	if jobList == nil {
		log.Infoln("Received an empty list of jobs:", jobList)
		return false, errors.New("received empty list of Jobs")
	}
	log.Debugln("Found", len(jobList.Items), "jobs")

	if debug { // Print out all the found jobs if debug logging is enabled.
		for _, job := range jobList.Items {
			log.Debugln(job.Name)
		}
	}

	// Iterate through Jobs and look for previous storage init jobs.
	for _, job := range jobList.Items {

		//TODO this is dumb hardcoding again
		if job.Name == checkStorageName+"-init-job" {
			log.Infoln("Found an old storage init job belonging to this check:", job.Name)
			return true, nil
		}

	}

	log.Infoln("Did not find any old storage(s) belonging to this check.")
	return false, nil
}

// findPreviousStorageCheckJob lists Jobs and checks their names and labels to determine if there should
// be an old storage check job belonging to this check that should be deleted.
func findPreviousStorageCheckJob() (bool, error) {

	log.Infoln("Attempting to find previously created storage check job belonging to this check.")

	jobList, err := client.BatchV1().Jobs(checkNamespace).List(metav1.ListOptions{})
	if err != nil {
		log.Infoln("error listing Jobs:", err)
		return false, err
	}
	if jobList == nil {
		log.Infoln("Received an empty list of jobs:", jobList)
		return false, errors.New("received empty list of Jobs")
	}
	log.Debugln("Found", len(jobList.Items), "jobs")

	if debug { // Print out all the found jobs if debug logging is enabled.
		for _, job := range jobList.Items {
			log.Debugln(job.Name)
		}
	}

	// Iterate through Jobs and look for previous storage init jobs.
	for _, job := range jobList.Items {

		//TODO this is dumb hardcoding again
		if job.Name == checkStorageName+"-check-job" {
			log.Infoln("Found an old storage init job belonging to this check:", job.Name)
			return true, nil
		}
	}

	log.Infoln("Did not find any old storace check jobs belonging to this check.")
	return false, nil
}

// waitForStorageToDelete waits for the service to be deleted.
func waitForStorageToDelete() chan bool {

	// Make and return a channel while we check that the service is gone in the background.
	deleteChan := make(chan bool, 1)

	go func() {
		defer close(deleteChan)
		for {
			_, err := client.CoreV1().PersistentVolumeClaims(checkNamespace).Get(checkStorageName, metav1.GetOptions{})
			if err != nil {
				log.Debugln("error from Storages().Get():", err.Error())
				if k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found") {
					log.Debugln("Storage deleted.")
					deleteChan <- true
					return
				}
			}
			time.Sleep(time.Millisecond * 250)
		}
	}()

	return deleteChan
}

// storageAvailable checks the status phase for "Bound" and returns a boolean.
// This will return a true if Phase is 'Bound'.
func storageAvailable(storage *corev1.PersistentVolumeClaim) bool {
	available := storage.Status.Phase
	if available == corev1.ClaimBound {
		return true
	}
	return false
}

// storageInitialized checks the status phase for "Bound" and returns a boolean.
// This will return a true if Phase is 'Bound'.
func storageInitialized(storage *batchv1.Job) bool {
	for _, condition := range storage.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			log.Infoln("Job is reporting", condition.Type, "with", condition.Status+".")
			return true
		}
	}
	return false
}
