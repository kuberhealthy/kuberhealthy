package metrics // import "github.com/kuberhealthy/kuberhealthy/v2/pkg/metrics"

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/health"
)

type PromMetricsConfig struct {
	SuppressErrorLabel  bool `yaml:"suppressErrorLabel,omitempty"`  // do we want to supress error label in metrics output(default: false)
	ErrorLabelMaxLength int  `yaml:"errorLabelMaxLength,omitempty"` // if not suppress, then bound the error label value length to a number of bytes
}

// promMetricName: helper fn for GenerateMetrics, does a quick format of the metric line - checkOrJob is literally the string "check" or "job"
func promMetricName(config PromMetricsConfig, checkOrJob string, checkName string, namespace string, status string, errors []string) string {
	metricName := fmt.Sprintf("kuberhealthy_%s{check=\"%s\",namespace=\"%s\",status=\"%s\"", checkOrJob, checkName, namespace, status)
	if !config.SuppressErrorLabel {
		errorsStr := ""
		if len(errors) > 0 {
			for _, error := range errors {
				errorsStr += fmt.Sprintf("%s|", error)
			}
			errorsStr = strings.TrimSuffix(errorsStr, "|")
			errorsStr = strings.ReplaceAll(errorsStr, "\"", "'")
		}
		if config.ErrorLabelMaxLength > 0 && len(errorsStr) > config.ErrorLabelMaxLength {
			errorsStr = errorsStr[0:config.ErrorLabelMaxLength]
		}
		metricName += fmt.Sprintf(",error=\"%s\"}", errorsStr)
	} else {
		metricName += "}"
	}
	return metricName
}

//GenerateMetrics takes the state and returns it in the Prometheus format
func GenerateMetrics(state health.State, config PromMetricsConfig) string {
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
		metricName := promMetricName(config, "check", c, d.Namespace, checkStatus, d.Errors)
		metricDurationName := fmt.Sprintf("kuberhealthy_check_duration_seconds{check=\"%s\",namespace=\"%s\"}", c, d.Namespace)
		metricCheckState[metricName] = checkStatus

		// if runDuration hasn't been set yet, ie. pod never ran or failed to provision, set runDuration to 0
		if d.RunDuration == "" {
			d.RunDuration = time.Duration(0).String()
		}
		runDuration, err := time.ParseDuration(d.RunDuration)
		if err != nil {
			log.Errorln("Error parsing run duration:", d.RunDuration, "for metric:", metricName, "error:", err)
		}
		metricCheckDuration[metricDurationName] = fmt.Sprintf("%f", runDuration.Seconds())
	}

	// Parse through all job details and append to metricState
	for c, d := range state.JobDetails {
		jobStatus := "0"
		if d.OK {
			jobStatus = "1"
		}
		metricName := promMetricName(config, "job", c, d.Namespace, jobStatus, d.Errors)
		metricDurationName := fmt.Sprintf("kuberhealthy_job_duration_seconds{check=\"%s\",namespace=\"%s\"}", c, d.Namespace)
		metricJobState[metricName] = jobStatus

		// if runDuration hasn't been set yet, ie. pod never ran or failed to provision, set runDuration to 0
		if d.RunDuration == "" {
			d.RunDuration = time.Duration(0).String()
		}
		runDuration, err := time.ParseDuration(d.RunDuration)
		if err != nil {
			log.Errorln("Error parsing run duration:", d.RunDuration, "for metric:", metricName, "error:", err)
		}
		metricJobDuration[metricDurationName] = fmt.Sprintf("%f", runDuration.Seconds())
	}

	// Add each metric format individually. This addresses issue https://github.com/kuberhealthy/kuberhealthy/issues/813.
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
