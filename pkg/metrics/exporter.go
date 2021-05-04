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

package metrics // import "github.com/Comcast/kuberhealthy/v2/pkg/metrics"

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/Comcast/kuberhealthy/v2/pkg/health"
)

//GenerateMetrics takes the state and returns it in the Prometheus format
func GenerateMetrics(state health.State) string {
	metricsOutput := ""
	healthStatus := "0"
	if state.OK {
		healthStatus = "1"
	}
	// Kuberhealthy metrics
	metricsOutput += "# HELP kuberhealthy_running Shows if kuberhealthy is running error free\n"
	metricsOutput += "# TYPE kuberhealthy_running gauge\n"
	metricsOutput += fmt.Sprintf("kuberhealthy_running{current_master=\"%s\"} 1\n", state.CurrentMaster)
	metricsOutput += "# HELP kuberhealthy_cluster_state Shows the status of the cluster\n"
	metricsOutput += "# TYPE kuberhealthy_cluster_state gauge\n"
	metricsOutput += fmt.Sprintf("kuberhealthy_cluster_state %s\n", healthStatus)

	metricCheckState := make(map[string]string)
	metricCheckDuration := make(map[string]string)
	metricJobState := make(map[string]string)
	metricJobDuration := make(map[string]string)

	// Parse through all check details and append to metricState
	for c, d := range state.CheckDetails {
		checkStatus := "0"
		if d.OK {
			checkStatus = "1"
		}
		errors := ""
		if len(d.Errors) > 0 {
			for _, error := range d.Errors {
				errors += fmt.Sprintf("%s|", error)
			}
		}
		errors = strings.ReplaceAll(errors, "\"", "'")
		metricName := fmt.Sprintf("kuberhealthy_check{check=\"%s\",namespace=\"%s\",status=\"%s\",error=\"%s\"}", c, d.Namespace, checkStatus, errors)
		metricDurationName := fmt.Sprintf("kuberhealthy_check_duration_seconds{check=\"%s\",namespace=\"%s\"}", c, d.Namespace)
		metricCheckState[metricName] = checkStatus
		runDuration, err := time.ParseDuration(d.RunDuration)
		if err != nil {
			log.Errorln("Error parsing run duration:", d.RunDuration, "for metric:", metricName, "error:", err)
		}
		// if runDuration hasn't been set yet, ie. pod never ran or failed to provision, set runDuration to 0
		if d.RunDuration == "" {
			runDuration = 0
		}
		metricCheckDuration[metricDurationName] = fmt.Sprintf("%f", runDuration.Seconds())
	}

	// Parse through all job details and append to metricState
	for c, d := range state.JobDetails {
		jobStatus := "0"
		if d.OK {
			jobStatus = "1"
		}
		errors := ""
		if len(d.Errors) > 0 {
			for _, error := range d.Errors {
				errors += fmt.Sprintf("%s|", error)
			}
		}
		metricName := fmt.Sprintf("kuberhealthy_job{check=\"%s\",namespace=\"%s\",status=\"%s\",error=\"%s\"}", c, d.Namespace, jobStatus, errors)
		metricDurationName := fmt.Sprintf("kuberhealthy_job_duration_seconds{check=\"%s\",namespace=\"%s\"}", c, d.Namespace)
		metricJobState[metricName] = jobStatus
		runDuration, err := time.ParseDuration(d.RunDuration)
		if err != nil {
			log.Errorln("Error parsing run duration:", d.RunDuration, "for metric:", metricName, "error:", err)
		}
		// if runDuration hasn't been set yet, ie. pod never ran or failed to provision, set runDuration to 0
		if d.RunDuration == "" {
			runDuration = 0
		}
		metricJobDuration[metricDurationName] = fmt.Sprintf("%f", runDuration.Seconds())
	}

	// Add each metric format individually. This addresses issue https://github.com/Comcast/kuberhealthy/issues/813.
	// Unless metric help and type are followed by the metric, datadog cannot process Kuberhealthy metrics.
	// Kuberhealthy check metrics
	metricsOutput += "# HELP kuberhealthy_check Shows the status of a Kuberhealthy check\n"
	metricsOutput += "# TYPE kuberhealthy_check gauge\n"
	for m, v := range metricCheckState {
		metricsOutput += fmt.Sprintf("%s %s\n", m, v)
	}
	metricsOutput += "# HELP kuberhealthy_check_duration_seconds Shows the check run duration of a Kuberhealthy check\n"
	metricsOutput += "# TYPE kuberhealthy_check_duration_seconds gauge\n"
	for m, v := range metricCheckDuration {
		metricsOutput += fmt.Sprintf("%s %s\n", m, v)
	}
	// Kuberhealthy job metrics
	metricsOutput += "# HELP kuberhealthy_job Shows the status of a Kuberhealthy job\n"
	metricsOutput += "# TYPE kuberhealthy_job gauge\n"
	for m, v := range metricJobState {
		metricsOutput += fmt.Sprintf("%s %s\n", m, v)
	}
	metricsOutput += "# HELP kuberhealthy_job_duration_seconds Shows the job run duration of a Kuberhealthy job\n"
	metricsOutput += "# TYPE kuberhealthy_job_duration_seconds gauge\n"
	for m, v := range metricJobDuration {
		metricsOutput += fmt.Sprintf("%s %s\n", m, v)
	}

	return metricsOutput
}

//ErrorStateMetrics is a Prometheus metric meant to show Kuberhealthy has error
func ErrorStateMetrics(state health.State) string {
	errorOutput := ""
	errorOutput += "# HELP kuberhealthy_running Shows if kuberhealthy is running error free\n"
	errorOutput += "# TYPE kuberhealthy_running gauge\n"
	errorOutput += fmt.Sprintf(`kuberhealthy_running{currentMaster="%s"} 0`, state.CurrentMaster)
	return errorOutput
}

// WriteMetricError handles errors in delivering metrics
func WriteMetricError(w http.ResponseWriter, state health.State) error {
	metricDefaultError := ErrorStateMetrics(state)
	_, err := w.Write([]byte(metricDefaultError))
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}
