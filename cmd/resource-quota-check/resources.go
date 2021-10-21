package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	kh "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Job struct containing namespace object used throughout code to grab all resources in all namespaces
type Job struct {
	namespace string
}

func runResourceQuotaCheck(ctx context.Context) {

	// List all namespaces in the cluster.
	allNamespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error occurred listing namespaces from the cluster: %v", err)
		reportErr := kh.ReportFailure([]string{err.Error()})
		if reportErr != nil {
			log.Fatalln("error reporting failure to kuberhealthy:", reportErr.Error())
		}
		return
	}

	select {
	case rqErrors := <-examineResourceQuotas(ctx, allNamespaces):
		if len(rqErrors) != 0 {
			log.Infoln("This check created", len(rqErrors), "errors and warnings.")
			log.Debugln("Errors and warnings:")
			for _, err := range rqErrors {
				log.Debugln(err)
			}
			log.Infoln("Reporting failures to kuberhealthy.")
			reportErr := kh.ReportFailure(rqErrors)
			if reportErr != nil {
				log.Fatalln("error reporting failures to kuberhealthy:", reportErr.Error())
			}
			return
		}
		log.Infoln("No errors or warnings were created during this check!")
	case <-ctx.Done():
		log.Infoln("Exiting and shutting down from interrupt.")
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
func examineResourceQuotas(ctx context.Context, namespaceList *v1.NamespaceList) chan []string {
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
			go createWorkerForNamespaceResourceQuotaCheck(ctx, job.namespace, results, &waitGroup)
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

// createWorkerForNamespaceResourceQuotaCheck looks at the resource quotas for a given namespace and creates error messages
// if usage is over a threshold.
/*
if blacklist is specified, and whitelist is not, then we simply operate on a blacklist
if blacklist is specified, and whitelist is also specified, then we operate on the whitelist unless the item is in the blacklist
if blacklist is not specified, but whitelist is, then we operate on a whitelist
if neither a blacklist or whitelist is specified, then all namespaces are targeted
*/
func createWorkerForNamespaceResourceQuotaCheck(ctx context.Context, namespace string, quotasChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer log.Debugln("worker for", namespace, "namespace is done!")

	// Prioritize blacklist over the whitelist.
	if len(blacklist) > 0 {
		if contains(namespace, blacklist) {
			log.Infoln("Skipping", namespace, "namespace (Blacklist).")
			return
		}
	}

	if len(whitelist) > 0 {
		if !contains(namespace, whitelist) {
			log.Infoln("Skipping", namespace, "namespace (Whitelist).")
			return
		}
	}

	examineResouceQuotasForNamespace(ctx, namespace, quotasChan)
}

// examineResouceQuotasForNamespace looks at resource quotas and sends error messages on threshold violations.
func examineResouceQuotasForNamespace(ctx context.Context, namespace string, c chan<- string) {
	log.Infoln("Looking at resource quotas for", namespace, "namespace.")
	quotas, err := client.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error occurred listing resource quotas for %s namespace %v", namespace, err)
		c <- err.Error()
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
			c <- err.Error()
		}
		if percentMemoryUsed >= threshold {
			err := fmt.Errorf("memory for %s namespace has reached threshold of %4.2f: USED: %d LIMIT: %d PERCENT_USED: %6.3f",
				namespace, threshold, status.Memory().MilliValue(), limits.Memory().MilliValue(), percentMemoryUsed)
			c <- err.Error()
		}
	}
}

// fillJobChan fills the job channel with namespace jobs.
func fillJobChan(namespaces *v1.NamespaceList, c chan<- *Job) {
	defer close(c)

	log.Infoln(len(namespaces.Items), "namespaces to look at.")
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
