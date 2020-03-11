package main

import (
	"fmt"
	"sync"
	"time"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Job struct {
	namespace string
}

func runResourceQuotaCheck() {

	// List all namespaces in the cluster.
	allNamespaces, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error occurred listing namespaces from the cluster: %v", err)
		reportErr := kh.ReportFailure([]string{err.Error()})
		if reportErr != nil {
			log.Fatalln("error reporting failure to kuberhealthy:", reportErr.Error())
		}
		return
	}

	select {
	case errors := <-examineResourceQuotas(allNamespaces):
		if len(errors) != 0 {
			log.Infoln("This check created", len(errors), "errors and warnings.")
			log.Debugln("Errors and warnings:")
			for _, err := range errors {
				log.Debugln(err)
			}
			log.Infoln("Reporting failures to kuberhealthy.")
			reportErr := kh.ReportFailure(errors)
			if reportErr != nil {
				log.Fatalln("error reporting failures to kuberhealthy:", reportErr.Error())
			}
			return
		}
		log.Infoln("No errors or warnings were created during this check!")
	case <-ctx.Done():
		log.Infoln("Canceling cleanup and shutting down from interrupt.")
		return
	case <-time.After(checkTimeLimit):
		err := fmt.Errorf("Check took too long and timed out.")
		log.Infoln("Reporting failure to kuberhealthy.")
		reportErr := kh.ReportFailure([]string{err.Error()})
		if reportErr != nil {
			log.Fatalln("error reporting failures to kuberhealthy:", reportErr.Error())
		}
		return
	}

	log.Infoln("Reporting success to kuberhealthy.")
	reportErr := kh.ReportSuccess()
	if reportErr != nil {
		log.Fatalln("error reporting success to kuberhealthy:", reportErr.Error())
	}
}

// examineResourceQuotas looks at the resource quotas and makes reports on namespaces that meet or pass the threshold.
func examineResourceQuotas(namespaceList *v1.NamespaceList) chan []string {
	resultChan := make(chan []string)

	resourceQuotasJobChan := make(chan *Job, len(namespaceList.Items))
	resourceQuotaErrorsChan := make(chan string, len(namespaceList.Items))

	go fillJobChan(namespaceList, resourceQuotasJobChan)

	go func(jobs chan *Job, results chan string) {

		errors := make([]string, 0)
		waitGroup := sync.WaitGroup{}

		for job := range jobs {
			waitGroup.Add(1)
			log.Debugln("Starting worker for", job.namespace, "namespace.")
			go checkResourceQuotaThresholdForNamespace(job.namespace, results, &waitGroup)
		}

		go func(wg *sync.WaitGroup) {
			log.Debugln("Waiting for workers to complete.")
			wg.Wait()
			log.Debugln("Workers done. Closing resource quota examination channel.")
			close(results)
		}(&waitGroup)

		for err := range results {
			errors = append(errors, err)
		}
		resultChan <- errors

		return
	}(resourceQuotasJobChan, resourceQuotaErrorsChan)

	return resultChan
}

// checkResourceQuotaThresholdForNamespace looks at the resource quotas for a given namespace and creates error messages
// if usage is over a threshold.
func checkResourceQuotaThresholdForNamespace(namespace string, quotasChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer log.Debugln("worker for", namespace, "namespace is done!")
	switch {
	// If whitelist and blacklist options are enabled, look at all whitelisted namespaces but the blacklisted namespaces.
	case whitelistOn && blacklistOn:
		// Skip non-whitelisted namespaces.
		if !contains(namespace, whitelistNamespaces) {
			log.Infoln("Skipping", namespace, "namespace.")
			return
		}
		// Skip blacklisted namespaces.
		if contains(nameespace, blacklistNamespaces) {
			log.Infoln("Skipping", namespace, "namespace.")
			return
		}
		if contains(namespace)
		log.Infoln("Looking at resource quotas for", namespace, "namespace.")
		quotas, err := client.CoreV1().ResourceQuotas(namespace).List(metav1.ListOptions{})
		if err != nil {
			err = fmt.Errorf("error occurred listing resource quotas for %s namespace %v", namespace, err)
			quotasChan <- err.Error()
			return
		}
		// Check if usage is at certain a threshold (percentage) of the limit.
		for _, rq := range quotas.Items {
			limits := rq.Status.Hard
			status := rq.Status.Used
			percentCPUUsed := float64(status.Cpu().MilliValue()) / float64(limits.Cpu().MilliValue())
			percentMemoryUsed := float64(status.Memory().MilliValue()) / float64(limits.Memory().MilliValue())
			log.Debugln("Current used for", namespace, "CPU:", status.Cpu().MilliValue(), "Memory:", status.Memory().MilliValue())
			log.Debugln("Limits for", namespace, "CPU:", limits.Cpu().MilliValue(), "Memory:", limits.Memory().MilliValue())
			if percentCPUUsed >= threshold {
				err := fmt.Errorf("cpu for %s namespace has reached threshold of %4.2f: USED: %d LIMIT: %d PERCENT_USED: %6.3f",
					namespace, threshold, status.Cpu().Value(), limits.Cpu().Value(), percentCPUUsed)
				quotasChan <- err.Error()
			}
			if percentMemoryUsed >= threshold {
				err := fmt.Errorf("memory for %s namespace has reached threshold of %4.2f: USED: %d LIMIT: %d PERCENT_USED: %6.3f",
					namespace, threshold, status.Memory().MilliValue(), limits.Memory().MilliValue(), percentMemoryUsed)
				quotasChan <- err.Error()
			}
		}
		return
	// If whitelist option is enabled, only look at the specified namespaces.
	case whitelistOn:
		// Skip non-whitelisted namespaces.
		if !contains(namespace, namespaces) {
			log.Infoln("Skipping", namespace, "namespace.")
			return
		}
		log.Infoln("Looking at resource quotas for", namespace, "namespace.")
		quotas, err := client.CoreV1().ResourceQuotas(namespace).List(metav1.ListOptions{})
		if err != nil {
			err = fmt.Errorf("error occurred listing resource quotas for %s namespace %v", namespace, err)
			quotasChan <- err.Error()
			return
		}
		// Check if usage is at certain a threshold (percentage) of the limit.
		for _, rq := range quotas.Items {
			limits := rq.Status.Hard
			status := rq.Status.Used
			percentCPUUsed := float64(status.Cpu().MilliValue()) / float64(limits.Cpu().MilliValue())
			percentMemoryUsed := float64(status.Memory().MilliValue()) / float64(limits.Memory().MilliValue())
			log.Debugln("Current used for", namespace, "CPU:", status.Cpu().MilliValue(), "Memory:", status.Memory().MilliValue())
			log.Debugln("Limits for", namespace, "CPU:", limits.Cpu().MilliValue(), "Memory:", limits.Memory().MilliValue())
			if percentCPUUsed >= threshold {
				err := fmt.Errorf("cpu for %s namespace has reached threshold of %4.2f: USED: %d LIMIT: %d PERCENT_USED: %6.3f",
					namespace, threshold, status.Cpu().Value(), limits.Cpu().Value(), percentCPUUsed)
				quotasChan <- err.Error()
			}
			if percentMemoryUsed >= threshold {
				err := fmt.Errorf("memory for %s namespace has reached threshold of %4.2f: USED: %d LIMIT: %d PERCENT_USED: %6.3f",
					namespace, threshold, status.Memory().MilliValue(), limits.Memory().MilliValue(), percentMemoryUsed)
				quotasChan <- err.Error()
			}
		}
		return
	// By default, use a blacklist.
	case blacklistOn:
		// Skip blacklisted namespaces.
		if contains(namespace, namespaces) {
			log.Infoln("Skipping", namespace, "namespace")
			return
		}
		log.Infoln("Looking at resource quotas for", namespace, "namespace.")
		quotas, err := client.CoreV1().ResourceQuotas(namespace).List(metav1.ListOptions{})
		if err != nil {
			err = fmt.Errorf("error occurred listing resource quotas for %s namespace %v", namespace, err)
			quotasChan <- err.Error()
			return
		}
		// Check if usage is at certain a threshold (percentage) of the limit.
		for _, rq := range quotas.Items {
			limits := rq.Status.Hard
			status := rq.Status.Used
			percentCPUUsed := float64(status.Cpu().MilliValue()) / float64(limits.Cpu().MilliValue())
			percentMemoryUsed := float64(status.Memory().MilliValue()) / float64(limits.Memory().MilliValue())
			log.Debugln("Current used for", namespace, "CPU:", status.Cpu().MilliValue(), "Memory:", status.Memory().MilliValue())
			log.Debugln("Limits for", namespace, "CPU:", limits.Cpu().MilliValue(), "Memory:", limits.Memory().MilliValue())
			if percentCPUUsed >= threshold {
				err := fmt.Errorf("cpu for %s namespace has reached threshold of %4.2f: USED: %d LIMIT: %d PERCENT_USED: %6.3f",
					namespace, threshold, status.Cpu().MilliValue(), limits.Cpu().MilliValue(), percentCPUUsed)
				quotasChan <- err.Error()
			}
			if percentMemoryUsed >= threshold {
				err := fmt.Errorf("memory for %s namespace has reached threshold of %4.2f: USED: %d LIMIT: %d PERCENT_USED: %6.3f",
					namespace, threshold, status.Memory().MilliValue(), limits.Memory().MilliValue(), percentMemoryUsed)
				quotasChan <- err.Error()
			}
		}
		return
	default:
		log.Infoln("Looking at resource quotas for", namespace, "namespace.")
		quotas, err := client.CoreV1().ResourceQuotas(namespace).List(metav1.ListOptions{})
		if err != nil {
			err = fmt.Errorf("error occurred listing resource quotas for %s namespace %v", namespace, err)
			quotasChan <- err.Error()
			return
		}
		// Check if usage is at certain a threshold (percentage) of the limit.
		for _, rq := range quotas.Items {
			limits := rq.Status.Hard
			status := rq.Status.Used
			percentCPUUsed := float64(status.Cpu().MilliValue()) / float64(limits.Cpu().MilliValue())
			percentMemoryUsed := float64(status.Memory().MilliValue()) / float64(limits.Memory().MilliValue())
			log.Debugln("Current used for", namespace, "CPU:", status.Cpu().MilliValue(), "Memory:", status.Memory().MilliValue())
			log.Debugln("Limits for", namespace, "CPU:", limits.Cpu().MilliValue(), "Memory:", limits.Memory().MilliValue())
			if percentCPUUsed >= threshold {
				err := fmt.Errorf("cpu for %s namespace has reached threshold of %4.2f: USED: %d LIMIT: %d PERCENT_USED: %6.3f",
					namespace, threshold, status.Cpu().MilliValue(), limits.Cpu().MilliValue(), percentCPUUsed)
				quotasChan <- err.Error()
			}
			if percentMemoryUsed >= threshold {
				err := fmt.Errorf("memory for %s namespace has reached threshold of %4.2f: USED: %d LIMIT: %d PERCENT_USED: %6.3f",
					namespace, threshold, status.Memory().MilliValue(), limits.Memory().MilliValue(), percentMemoryUsed)
				quotasChan <- err.Error()
			}
		}
		return
	}
}

// fillJobChan fills the job channel with namespace jobs.
func fillJobChan(namespaces *v1.NamespaceList, c chan<- *Job) {
	defer close(c)

	log.Infoln(len(namespaces.Items), "namespacese to look at.")
	for _, ns := range namespaces.Items {
		log.Debugln("Creating job for", ns.GetName(), "namespace.")
		c <- &Job{
			namespace: ns.GetName(),
		}
	}

	return
}

// contains returns a boolean value based on whether or not a slice of strings contains
// a string.
func contains(s string, list []string) bool {
	for _, str := range list {
		if s == str {
			return true
		}
	}
	return false
}
