package main

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

// serviceAccountNamespacePath records the location of the namespace file written by
// Kubernetes. Tests override this path to supply fixture data.
var serviceAccountNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// containsString returns a boolean value based on whether or not a slice of strings contains
// a string.
func containsString(s string, list []string) bool {
	// walk through each entry looking for the provided string
	for _, str := range list {
		if s == str {
			return true
		}
	}

	return false
}

// GetMyNamespace fetches the pod's local namespace from Kubernetes. If none can be determined, the supplied default is used.
func GetMyNamespace(defaultNamespace string) string {

	instanceNamespace := defaultNamespace

	// read the namespace file so we can determine the runtime namespace
	data, err := os.ReadFile(serviceAccountNamespacePath)
	if err != nil {
		log.Warnln("Failed to open namespace file:", err.Error())
	}

	// trim whitespace from the namespace so newline terminated files still parse correctly
	if len(data) != 0 {
		trimmedNamespace := strings.TrimSpace(string(data))
		if len(trimmedNamespace) != 0 {
			log.Infoln("Found instance namespace:", trimmedNamespace)
			return trimmedNamespace
		}
	}

	log.Warnln("Did not find instance namespace. Using default namespace:", defaultNamespace)

	return instanceNamespace
}

// GetMyHostname returns the pod name if present, or the system hostname.
// When neither can be determined, the supplied default is used instead.
func GetMyHostname(defaultHostname string) string {

	if podName := os.Getenv("POD_NAME"); podName != "" {
		log.Infoln("Found pod name:", podName)
		return podName
	}

	host, err := os.Hostname()
	if err != nil || host == "" {
		log.Warnln("Failed to determine hostname. Using default:", defaultHostname)
		return defaultHostname
	}
	log.Warnln("POD_NAME environment variable not set. Using system hostname:", host)
	return host
}
