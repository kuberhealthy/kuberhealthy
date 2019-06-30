package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"

	"github.com/Comcast/kuberhealthy/pkg/checks/external"
)

// newExternalTestCheck creates a new external test checker struct with a basic set of defaults
// that work out of the box
func newExternalTestCheck() (*external.Checker, error) {
	podCheckFile := "test/basicCheckerPod.yaml"
	p, err := loadTestPodSpecFile(podCheckFile)
	if err != nil {
		return &external.Checker{}, errors.New("Unable to load kubernetes pod spec " + podCheckFile + " " + err.Error())
	}
	return newTestCheckFromSpec(p), nil
}

// newTestCheckFromSpec creates a new test checker but using the supplied
// spec file for pods
func newTestCheckFromSpec(spec *apiv1.PodSpec) *external.Checker {
	// create a new checker and insert this pod spec
	checker := external.New(spec) // external checker does not ever return an error so we drop it
	checker.Namespace = "kuberhealthy"
	checker.Debug = true
	return checker
}

// loadTestPodSpecFile loads a pod spec yaml from disk in this
// directory and returns the pod spec struct it represents
func loadTestPodSpecFile(path string) (*apiv1.PodSpec, error) {

	podSpec := apiv1.PodSpec{}

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
