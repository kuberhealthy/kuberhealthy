package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func runResourceQuotaCheck() {

	// List all namespaces in the cluster.
	allNamespaces, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		log.Errorln("error occurred listing namespaces from the cluster:", err.Error())
	}

	// For every namespace listed:
	for _, ns := range allNamespaces.Items {
		switch {
		// If whitelist option is enabled, only look at the specified namespaces.
		case whitelistOn:
			if !contains(ns.GetName(), namespaces) {
				continue
			}
			quotas, err := client.CoreV1().ResourceQuotas(ns.GetName()).List(metav1.ListOptions{})
			if err != nil {
				// log.Errorln("error occurred listing resource quotas for", ns, "namespace:", err.Error())
				err = fmt.Errorf("error occurred listing resource quotas for %s namespace %v", ns.GetName(), err)
				checkErrors = append(checkErrors, err.Error())
			}
			for _, rq := range quotas.Items {
				limits := rq.Status.Hard
				status := rq.Status.Used
				if float64(status.Cpu().Value()) >= float64(limits.Cpu().Value())*threshold {
					err := fmt.Errorf("usage threshold for CPU for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %5.2f",
						ns.GetName(), status.Cpu().Value(), limits.Cpu().Value(), float64(status.Cpu().Value()/limits.Cpu().Value()))
					checkErrors = append(checkErrors, err.Error())
				}
				if float64(status.Memory().Value()) >= float64(limits.Memory().Value())*threshold {
					err := fmt.Errorf("usage threshold for memory for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %5.2f",
						ns.GetName(), status.Memory().Value(), limits.Memory().Value(), float64(status.Memory().Value()/limits.Memory().Value()))
					checkErrors = append(checkErrors, err.Error())
				}
			}
		// By default, use a blacklist.
		default:
			if contains(ns.GetName(), namespaces) {
				continue
			}
			quotas, err := client.CoreV1().ResourceQuotas(ns.GetName()).List(metav1.ListOptions{})
			if err != nil {
				// log.Errorln("error occurred listing resource quotas for", ns, "namespace:", err.Error())
				err = fmt.Errorf("error occurred listing resource quotas for %s namespace %v", ns.GetName(), err)
				checkErrors = append(checkErrors, err.Error())
			}
			for _, rq := range quotas.Items {
				limits := rq.Status.Hard
				status := rq.Status.Used
				if float64(status.Cpu().Value()) >= float64(limits.Cpu().Value())*threshold {
					err := fmt.Errorf("usage threshold for CPU for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %5.2f",
						ns.GetName(), status.Cpu().Value(), limits.Cpu().Value(), float64(status.Cpu().Value()/limits.Cpu().Value()))
					checkErrors = append(checkErrors, err.Error())
				}
				if float64(status.Memory().Value()) >= float64(limits.Memory().Value())*threshold {
					err := fmt.Errorf("usage threshold for memory for %s namespace has been met: USED: %d LIMIT: %d PERCENT_USED: %5.2f",
						ns.GetName(), status.Memory().Value(), limits.Memory().Value(), float64(status.Memory().Value()/limits.Memory().Value()))
					checkErrors = append(checkErrors, err.Error())
				}
			}
		}
	}

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
