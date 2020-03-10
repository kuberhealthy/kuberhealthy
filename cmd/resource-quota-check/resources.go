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
		wg := sync.WaitGroup{}

		go func() {
			log.Debugln("Waiting for workers to complete.")
			wg.Wait()
			log.Debugln("Workers done. Closing resource quota examination channel.")
			close(results)
		}()

		for job := range jobs {
			wg.Add(1)
			log.Debugln("Starting worker for", job.namespace, "namespace.")
			go checkResourceQuotaThresholdForNamespace(job.namespace, results, &wg)
			// time.Sleep(time.Millisecond)
		}

		for err := range results {
			errors = append(errors, err)
		}
		resultChan <- errors

		return
	}(resourceQuotasJobChan, resourceQuotaErrorsChan)

	return resultChan
}

// func checkResourceQuotaThresholdForNamespace(namespace *v1.Namespace, quotasChan chan string, wg *sync.WaitGroup) {
func checkResourceQuotaThresholdForNamespace(namespace string, quotasChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	ns := namespace.GetName()
	// defer log.Debugln("worker for", namespace.GetName(), "namespace is done!")
	defer log.Debugln("worker for", ns, "namespace is done!")
	switch {
	// If whitelist option is enabled, only look at the specified namespaces.
	case whitelistOn:
		// if !contains(namespace.GetName(), namespaces) {
		// 	log.Infoln("Skipping", namespace.GetName(), "namespace")
		// 	return
		// }
		if !contains(ns, namespaces) {
			log.Infoln("Skipping", ns, "namespace")
			return
		}
		// log.Infoln("Looking at resource quotas for", namespace.GetName(), "namespace.")
		// quotas, err := client.CoreV1().ResourceQuotas(namespace.GetName()).List(metav1.ListOptions{})
		log.Infoln("Looking at resource quotas for", ns, "namespace.")
		quotas, err := client.CoreV1().ResourceQuotas(ns).List(metav1.ListOptions{})
		if err != nil {
			err = fmt.Errorf("error occurred listing resource quotas for %s namespace %v", namespace.GetName(), err)
			quotasChan <- err.Error()
			return
		}
		log.Infoln("Quotas:", len(quotas.Items))
		for _, rq := range quotas.Items {
			log.Infoln(rq)
			limits := rq.Status.Hard
			status := rq.Status.Used
			log.Debugln("Current used for", namespace.GetName(), "CPU:", status.Cpu().Value(), "Memory:", status.Memory().Value())
			log.Debugln("Limits for", namespace.GetName(), "CPU:", limits.Cpu().Value(), "Memory:", limits.Memory().Value())
			log.Debugln(float64(status.Cpu().Value()), float64(limits.Cpu().Value()), float64(limits.Cpu().Value())*threshold)
			log.Debugf("%5.2f %5.2f\n", float64(status.Cpu().Value()*100/limits.Cpu().Value()), float64(status.Cpu().Value()/limits.Cpu().Value()))
			log.Infoln("Cpu.Value:", status.Cpu().Value())
			log.Infoln("int64 of float64(limits.Cpu.Value) * threshold:", int64(float64(limits.Cpu().Value())*threshold))
			if status.Cpu().Value() >= int64(float64(limits.Cpu().Value())*threshold) {
				// err := fmt.Errorf("usage threshold for CPU for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %5.2f",
				// 	namespace.GetName(), status.Cpu().Value(), limits.Cpu().Value(), float64(status.Cpu().Value()*100/limits.Cpu().Value()))
				err := fmt.Errorf("usage threshold for CPU for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %10.10f",
					ns, status.Cpu().Value(), limits.Cpu().Value(), float64(status.Cpu().Value()*100/limits.Cpu().Value()))
				quotasChan <- err.Error()
			}
			if float64(status.Memory().Value()) >= float64(limits.Memory().Value())*threshold {
				// err := fmt.Errorf("usage threshold for memory for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %5.2f",
				// 	namespace.GetName(), status.Memory().Value(), limits.Memory().Value(), float64(status.Memory().Value()*100/limits.Memory().Value()))
				err := fmt.Errorf("usage threshold for memory for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %10.10f",
					ns, status.Memory().Value(), limits.Memory().Value(), float64(status.Memory().Value()*100/limits.Memory().Value()))
				quotasChan <- err.Error()
			}
		}
		return
	// By default, use a blacklist.
	default:
		if contains(ns, namespaces) {
			log.Infoln("Skipping", ns, "namespace")
			return
		}
		// if contains(namespace.GetName(), namespaces) {
		// 	log.Infoln("Skipping", namespace.GetName(), "namespace")
		// 	return
		// }
		// quotas, err := client.CoreV1().ResourceQuotas(namespace.GetName()).List(metav1.ListOptions{})
		log.Infoln("Looking at resource quotas for", ns, "namespace.")
		quotas, err := client.CoreV1().ResourceQuotas(ns).List(metav1.ListOptions{})
		if err != nil {
			// err = fmt.Errorf("error occurred listing resource quotas for %s namespace %v", namespace.GetName(), err)
			err = fmt.Errorf("error occurred listing resource quotas for %s namespace %v", ns, err)
			quotasChan <- err.Error()
			return
		}
		log.Infoln("Quotas:", len(quotas.Items))
		for _, rq := range quotas.Items {
			log.Infoln(rq)
			limits := rq.Status.Hard
			status := rq.Status.Used
			log.Debugln("Current used for", ns, "CPU:", status.Cpu().MilliValue(), "Memory:", status.Memory().MilliValue())
			log.Debugln("Limits for", ns, "CPU:", limits.Cpu().MilliValue(), "Memory:", limits.Memory().MilliValue())
			log.Debugln(float64(status.Cpu().MilliValue()), float64(limits.Cpu().MilliValue()), float64(limits.Cpu().MilliValue())*threshold)
			log.Debugf("%f %f\n", float64(status.Cpu().MilliValue()*100/limits.Cpu().MilliValue()), float64(status.Cpu().MilliValue()/limits.Cpu().MilliValue()))
			log.Infoln("Cpu.Value:", status.Cpu().Value())
			log.Infoln("Cpu.MilliValue:", status.Cpu().MilliValue())
			log.Infoln("int64 of float64(limits.Cpu.Value) * threshold:", int64(float64(limits.Cpu().Value())*threshold))
			if float64(status.Cpu().Value()) >= float64(limits.Cpu().Value())*threshold {
				// err := fmt.Errorf("usage threshold for CPU for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %5.2f",
				// 	namespace.GetName(), status.Cpu().Value(), limits.Cpu().Value(), float64(status.Cpu().Value()*100/limits.Cpu().Value()))
				err := fmt.Errorf("usage threshold for CPU for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %10.10f",
					ns, status.Cpu().MilliValue(), limits.Cpu().MilliValue(), float64(status.Cpu().MilliValue()*100/limits.Cpu().MilliValue()))
				quotasChan <- err.Error()
			}
			if float64(status.Memory().Value()) >= float64(limits.Memory().Value())*threshold {
				// err := fmt.Errorf("usage threshold for memory for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %5.2f",
				// 	namespace.GetName(), status.Memory().Value(), limits.Memory().Value(), float64(status.Memory().Value()*100/limits.Memory().Value()))
				err := fmt.Errorf("usage threshold for memory for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %10.10f",
					ns, status.Memory().MilliValue(), limits.Memory().MilliValue(), float64(status.Memory().MilliValue()*100/limits.Memory().MilliValue()))
				quotasChan <- err.Error()
			}
		}
		return
	}
}

// fillJobChan fills the job channel with namespace jobs.
func fillJobChan(namespaces *v1.NamespaceList, c chan<- *Job) {
	defer close(c)

	for _, ns := range namespaces.Items {
		log.Infoln("Creating job for", ns.GetName(), "namespace.")
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
