/* Copyright 2018 Comcast Cable Communications Management, LLC
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at
       http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package metrics // import "github.com/Comcast/kuberhealthy/pkg/metrics"

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/Comcast/kuberhealthy/pkg/health"
)

//GenerateMetrics takes the state and returns it in the Prometheus format
func GenerateMetrics(state health.State) string {
	metricsOutput := ""
	healthStatus := "0"
	if state.OK {
		healthStatus = "1"
	}
	metricsOutput += "# HELP kuberhealthy_running Shows if kuberhealthy is running error free\n"
	metricsOutput += "# TYPE kuberhealthy_running gauge\n"
	metricsOutput += fmt.Sprintf("kuberhealthy_running{currentMaster=\"%s\"} 1\n", state.CurrentMaster)
	metricsOutput += "# HELP kuberhealthy_cluster_state Shows the status of the cluster\n"
	metricsOutput += "# TYPE kuberhealthy_cluster_state gauge\n"
	metricsOutput += fmt.Sprintf("kuberhealthy_cluster_state %s\n", healthStatus)
	checkMetricState := map[string]string{}
	for c, d := range state.CheckDetails {
		metricName := fmt.Sprintf("kuberhealthy_check{check=\"%s\",namespace=\"%s\"}", c, d.Namespace)
		checkStatus := "0"
		if d.OK {
			checkStatus = "1"
		}
		checkMetricState[metricName] = checkStatus
	}
	metricsOutput += "# HELP kuberhealthy_check Shows the status of a Kuberhealthy check\n"
	metricsOutput += "# TYPE kuberhealthy_check gauge\n"
	for m, v := range checkMetricState {
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
