// Copyright 2020 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

type crdName struct {
	Singular string
	Plural   string
}

type crdGenerator struct {
	ControllerGenOpts string
	YAMLDir           string
	CRDNames          []crdName
	CRDAPIGroup       string
	ControllerPath    string
	CustomizeYAML     func(crdGenerator) error
}

var (
	controllergen string
	gojsontoyaml  string

	crdGenerators = []crdGenerator{
		{
			ControllerGenOpts: "crd:crdVersions=v1,preserveUnknownFields=false",
			YAMLDir:           "./generated",
			CRDAPIGroup:       "comcast.github.io",
			ControllerPath:    "../pkg/apis/khcheck/v1",
			CRDNames: []crdName{
				{"khcheck", "khchecks"},
			},
			CustomizeYAML: func(generator crdGenerator) error {
				return nil
			},
		},
		{
			ControllerGenOpts: "crd:crdVersions=v1,preserveUnknownFields=false",
			YAMLDir:           "./generated",
			CRDAPIGroup:       "comcast.github.io",
			ControllerPath:    "../pkg/apis/khjob/v1",
			CRDNames: []crdName{
				{"khjob", "khjobs"},
			},
			CustomizeYAML: func(generator crdGenerator) error {
				return nil
			},
		},
		{
			ControllerGenOpts: "crd:crdVersions=v1",
			YAMLDir:           "./generated",
			CRDAPIGroup:       "comcast.github.io",
			ControllerPath:    "../pkg/apis/khstate/v1",
			CRDNames: []crdName{
				{"khstate", "khstates"},
			},
			CustomizeYAML: func(generator crdGenerator) error {
				return nil
			},
		},
	}
)

func (generator crdGenerator) generateYAMLManifests() error {
	outputDir, err := filepath.Abs(generator.YAMLDir)
	if err != nil {
		return errors.Wrapf(err, "absolute CRD output path %s", generator.YAMLDir)
	}

	// #nosec
	log.Println("running binary:", controllergen)
	cmd := exec.Command(controllergen,
		generator.ControllerGenOpts,
		"paths=.",
		"output:crd:dir="+outputDir,
	)

	cmd.Dir, err = filepath.Abs(generator.ControllerPath)
	if err != nil {
		return errors.Wrapf(err, "absolute controller path %s", generator.ControllerPath)
	}

	cmdOutput, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalln(err)
	}

	cmdOutputErr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalln(err)
	}

	go io.Copy(os.Stdout, cmdOutput)
	go io.Copy(os.Stderr, cmdOutputErr)

	err = cmd.Run()
	if err != nil {
		log.Printf("failed to run command %v", err.Error())
		return err
	}

	if generator.CustomizeYAML == nil {
		return nil
	}

	err = generator.CustomizeYAML(generator)
	if err != nil {
		return errors.Wrap(err, "customizing YAML")
	}

	return nil
}

func main() {
	flag.StringVar(&controllergen, "controller-gen", "controller-gen", "controller-gen binary path")
	flag.StringVar(&gojsontoyaml, "gojsontoyaml", "gojsontoyaml", "gojsontoyaml binary path")
	flag.Parse()

	for _, generator := range crdGenerators {
		err := generator.generateYAMLManifests()
		if err != nil {
			log.Fatalf("generating YAML manifests: %v", err)
		}
	}
}
