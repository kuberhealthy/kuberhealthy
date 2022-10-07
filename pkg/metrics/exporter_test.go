package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"

	khstatev1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khstate/v1"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/health"
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
	result := GenerateMetrics(state, PromMetricsConfig{})
	metrics := parseMetrics(result)
	if metrics[`kuberhealthy_running{current_master=""}`] != "1" {
		t.Fatal("Kuberhealthy is not shown as running")
	}
	if metrics["kuberhealthy_cluster_state"] == "1" {
		t.Fatal("Kuberhealthy shows cluster as healthy when it isn't")
	}
	// Test OK state
	state = health.State{
		OK: true,
	}
	result = GenerateMetrics(state, PromMetricsConfig{})
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_running{current_master=""}`] != "1" {
		t.Fatal("Kuberhealthy is not shown as running")
	}
	if metrics["kuberhealthy_cluster_state"] != "1" {
		t.Fatal("Kuberhealthy shows cluster as not healthy when it is")
	}
	// Test not OK state
	state = health.State{
		OK: false,
	}
	result = GenerateMetrics(state, PromMetricsConfig{})
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_running{current_master=""}`] != "1" {
		t.Fatal("Kuberhealthy is not shown as running")
	}
	if metrics["kuberhealthy_cluster_state"] == "1" {
		t.Fatal("Kuberhealthy shows cluster as healthy when it isn't")
	}
	// Test State with master
	state = health.State{
		CurrentMaster: "testMaster",
	}
	result = GenerateMetrics(state, PromMetricsConfig{})
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_running{current_master="testMaster"}`] != "1" {
		t.Fatal("Kuberhealthy is not shown as running")
	}
	if metrics["kuberhealthy_cluster_state"] == "1" {
		t.Fatal("Kuberhealthy shows cluster as healthy when it isn't")
	}
	// Test with checks, one good, one bad
	state = health.State{
		CheckDetails: map[string]khstatev1.WorkloadDetails{
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
	result = GenerateMetrics(state, PromMetricsConfig{})
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_running{current_master=""}`] != "1" {
		t.Fatal("Kuberhealthy is not shown as running")
	}
	if metrics["kuberhealthy_cluster_state"] == "1" {
		t.Fatal("Kuberhealthy shows cluster as healthy when it isn't")
	}
	if metrics[`kuberhealthy_check{check="good",namespace="",status="1",error=""}`] != "1" {
		t.Fatal("Kuberhealthy good check shows as bad")
	}
	if metrics[`kuberhealthy_check{check="bad",namespace="",status="0",error=""}`] != "0" {
		t.Fatal("Kuberhealthy good check shows as bad")
	}
	if metrics[`kuberhealthy_check{check="",namespace="",status="1",error=""}`] != "1" {
		t.Fatal("Kuberhealthy good check shows as bad")
	}
	state = health.State{
		CheckDetails: map[string]khstatev1.WorkloadDetails{
			"bad": {
				Errors: []string{"12345678910"},
			},
		},
	}
	result = GenerateMetrics(state, PromMetricsConfig{})
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_check{check="bad",namespace="",status="0",error="12345678910"}`] != "0" {
		t.Fatal("Kuberhealthy bad error label check does not match - test 1", metrics)
	}
	result = GenerateMetrics(state, PromMetricsConfig{SuppressErrorLabel: true})
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_check{check="bad",namespace="",status="0"}`] != "0" {
		t.Fatal("Kuberhealthy bad error label check does not match - test 2", metrics)
	}
	result = GenerateMetrics(state, PromMetricsConfig{SuppressErrorLabel: false, ErrorLabelMaxLength: 4})
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_check{check="bad",namespace="",status="0",error="1234"}`] != "0" {
		t.Fatal("Kuberhealthy bad error label check does not match - test 3", metrics)
	}
	state = health.State{
		CheckDetails: map[string]khstatev1.WorkloadDetails{
			"bad": {
				Errors: []string{"123"},
			},
		},
	}
	result = GenerateMetrics(state, PromMetricsConfig{SuppressErrorLabel: false, ErrorLabelMaxLength: 10})
	metrics = parseMetrics(result)
	if metrics[`kuberhealthy_check{check="bad",namespace="",status="0",error="123"}`] != "0" {
		t.Fatal("Kuberhealthy bad error label check does not match - test 4", metrics)
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
