// Copyright 2018 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Comcast/kuberhealthy/v2/pkg/health"
)

func parseMetrics(metricOutput string) map[string]string {
	metricMap := map[string]string{}
	lines := strings.Split(metricOutput, "\n")
	for _, l := range lines {
		if l == "" || l[0] == '#' {
			continue
		}
		metric := strings.Split(l, " ")
		metricMap[metric[0]] = metric[1]
	}
	return metricMap
}
func TestGenerateMetrics(t *testing.T) {
	// Test Empty State
	state := health.State{}
	result := GenerateMetrics(state)
	metrics := parseMetrics(result)
	if metrics[`kuberhealthy_running{currentMaster=""}`] != "1" {
		t.Fatal("Kuberhealthy is not shown as running")
	}
	if metrics["kuberhealthy_cluster_state"] == "1" {
		t.Fatal("Kuberhealthy shows cluster as healthy when it isn't")
	}
	// Test OK state
	state = health.State{
		OK: true,
	}
	result = GenerateMetrics(state)
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_running{currentMaster=""}`] != "1" {
		t.Fatal("Kuberhealthy is not shown as running")
	}
	if metrics["kuberhealthy_cluster_state"] != "1" {
		t.Fatal("Kuberhealthy shows cluster as not healthy when it is")
	}
	// Test not OK state
	state = health.State{
		OK: false,
	}
	result = GenerateMetrics(state)
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_running{currentMaster=""}`] != "1" {
		t.Fatal("Kuberhealthy is not shown as running")
	}
	if metrics["kuberhealthy_cluster_state"] == "1" {
		t.Fatal("Kuberhealthy shows cluster as healthy when it isn't")
	}
	// Test State with master
	state = health.State{
		CurrentMaster: "testMaster",
	}
	result = GenerateMetrics(state)
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_running{currentMaster="testMaster"}`] != "1" {
		t.Fatal("Kuberhealthy is not shown as running")
	}
	if metrics["kuberhealthy_cluster_state"] == "1" {
		t.Fatal("Kuberhealthy shows cluster as healthy when it isn't")
	}
	// Test with checks, one good, one bad
	state = health.State{
		CheckDetails: map[string]health.CheckDetails{
			"good": {
				OK: true,
			},
			"bad": {
				OK: false,
			},
			"": {
				OK: true,
			},
		},
	}
	result = GenerateMetrics(state)
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_running{currentMaster=""}`] != "1" {
		t.Fatal("Kuberhealthy is not shown as running")
	}
	if metrics["kuberhealthy_cluster_state"] == "1" {
		t.Fatal("Kuberhealthy shows cluster as healthy when it isn't")
	}
	if metrics[`kuberhealthy_check{check="good"}`] != "1" {
		t.Fatal("Kuberhealthy good check shows as bad")
	}
	if metrics[`kuberhealthy_check{check="bad"}`] != "0" {
		t.Fatal("Kuberhealthy good check shows as bad")
	}
	if metrics[`kuberhealthy_check{check=""}`] != "1" {
		t.Fatal("Kuberhealthy good check shows as bad")
	}
}

func TestErrorStateMetrics(t *testing.T) {
	state := health.State{
		CurrentMaster: "testMaster",
	}
	errorState := ErrorStateMetrics(state)
	lines := strings.Split(errorState, "\n")
	metricValue := strings.Split(lines[2], " ")
	if lines[2] != `kuberhealthy_running{currentMaster="testMaster"} 0` {
		t.Fatal("Error State Metrics does not match expected value")
	}
	if metricValue[1] != "0" {
		t.Fatal("Error State Metric is not 0")
	}
	state = health.State{}
	errorState = ErrorStateMetrics(state)
	lines = strings.Split(errorState, "\n")
	metricValue = strings.Split(lines[2], " ")
	if lines[2] != `kuberhealthy_running{currentMaster=""} 0` {
		t.Fatal("Error State Metrics does not match expected value")
	}
	if metricValue[1] != "0" {
		t.Fatal("Error State Metric is not 0")
	}
}
func TestWriteMetricError(t *testing.T) {
	recorder := httptest.NewRecorder()
	state := health.State{
		CurrentMaster: "testMaster",
	}
	err := WriteMetricError(recorder, state)
	if err != nil {
		t.Fatal("Error occurred writing metric error: ", err)
	}
	if recorder.Body.String() != ErrorStateMetrics(state) {
		t.Fatal("Error Metric does not match actual error metric function")
	}
	recorder = httptest.NewRecorder()
	state = health.State{}
	err = WriteMetricError(recorder, state)
	if err != nil {
		t.Fatal("Error occurred writing metric error: ", err)
	}
	if recorder.Body.String() != ErrorStateMetrics(state) {
		t.Fatal("Error Metric does not match actual error metric function")
	}
}
