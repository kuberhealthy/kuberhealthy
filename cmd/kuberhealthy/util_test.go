package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external"
	"github.com/Comcast/kuberhealthy/v2/pkg/khcheckcrd"
)

const defaultNamespace = "kuberhealthy"

// newExternalTestCheck creates a new external test checker struct with a basic set of defaults
// that work out of the box
func newExternalTestCheck(c *kubernetes.Clientset) (*external.Checker, error) {
	podCheckFile := "test/basicCheckerPod.yaml"
	p, err := loadTestPodSpecFile(podCheckFile)
	if err != nil {
		return &external.Checker{}, errors.New("Unable to load kubernetes pod spec " + podCheckFile + " " + err.Error())
	}
	return newTestCheckFromSpec(c, p), nil
}

// newTestCheckFromSpec creates a new test checker but using the supplied
// spec file for pods
func newTestCheckFromSpec(c *kubernetes.Clientset, spec *khcheckcrd.KuberhealthyCheck) *external.Checker {
	// create a new checker and insert this pod spec
	checker := external.New(c, spec, khCheckClient, khStateClient, externalCheckReportingURL) // external checker does not ever return an error so we drop it
	checker.Debug = true
	return checker
}

// loadTestPodSpecFile loads a pod spec yaml from disk in this
// directory and returns the pod spec struct it represents
func loadTestPodSpecFile(path string) (*khcheckcrd.KuberhealthyCheck, error) {

	podSpec := khcheckcrd.KuberhealthyCheck{}

	// open the yaml file
	f, err := os.Open(path)
	if err != nil {
		return &podSpec, err
	}

	// read in all the configuration bytes
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return &podSpec, err
	}

	log.Debugln("Decoding this YAML:", string(b))
	j, err := yaml.YAMLToJSON(b)

	// unmarshal the pod into the pod struct and return
	err = json.Unmarshal(j, &podSpec)
	return &podSpec, err
}
