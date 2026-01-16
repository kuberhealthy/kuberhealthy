package metrics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/health"
)

type PromMetricsConfig struct {
	SuppressErrorLabel  bool     `yaml:"suppressErrorLabel,omitempty"`  // do we want to supress error label in metrics output(default: false)
	ErrorLabelMaxLength int      `yaml:"errorLabelMaxLength,omitempty"` // if not suppress, then bound the error label value length to a number of bytes
	LabelAllowlist      []string `yaml:"labelAllowlist,omitempty"`      // allowlisted labels for inclusion in metrics output
	LabelDenylist       []string `yaml:"labelDenylist,omitempty"`       // denylisted labels for exclusion from metrics output
	LabelValueMaxLength int      `yaml:"labelValueMaxLength,omitempty"` // bound extra label values to a maximum byte length
}

// buildLabelSet converts a list into a lookup map for faster checks.
func buildLabelSet(values []string) map[string]struct{} {
	// Return an empty map when no values are supplied.
	if len(values) == 0 {
		return map[string]struct{}{}
	}

	// Build the set for constant-time lookups.
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}

	return set
}

// labelAllowed determines whether an extra label should be emitted.
func labelAllowed(label string, allowlist map[string]struct{}, denylist map[string]struct{}) bool {
	// Enforce allowlist when provided.
	if len(allowlist) > 0 {
		_, allowed := allowlist[label]
		if !allowed {
			return false
		}
	}

	// Enforce denylist when provided.
	if len(denylist) > 0 {
		_, denied := denylist[label]
		if denied {
			return false
		}
	}

	return true
}

// sanitizeLabelValue makes a label safe for Prometheus output.
func sanitizeLabelValue(value string, maxLength int) string {
	// Normalize whitespace and quotes to avoid invalid output.
	sanitized := strings.ReplaceAll(value, "\"", "'")
	sanitized = strings.ReplaceAll(sanitized, "\n", " ")
	sanitized = strings.ReplaceAll(sanitized, "\r", " ")

	// Enforce maximum length when configured.
	if maxLength > 0 && len(sanitized) > maxLength {
		sanitized = sanitized[0:maxLength]
	}

	return sanitized
}

// errorLabelValue formats errors into a single label string.
func errorLabelValue(errors []string, maxLength int) string {
	// Skip formatting when there are no errors.
	if len(errors) == 0 {
		return ""
	}

	// Build a pipe-delimited string of errors.
	joined := strings.Join(errors, "|")

	// Sanitize and truncate the error payload.
	return sanitizeLabelValue(joined, maxLength)
}

// baseCheckLabels returns the common labels for per-check metrics.
func baseCheckLabels(checkName string, namespace string) map[string]string {
	// Build the fixed label set for all check metrics.
	return map[string]string{
		"check":     checkName,
		"namespace": namespace,
	}
}

// copyLabels duplicates a label map so mutations are safe.
func copyLabels(source map[string]string) map[string]string {
	// Preserve capacity for the copied map.
	labels := make(map[string]string, len(source))
	for key, value := range source {
		labels[key] = value
	}
	return labels
}

// addExtraLabel adds a label when it passes allowlist/denylist filters.
func addExtraLabel(labels map[string]string, key string, value string, allowlist map[string]struct{}, denylist map[string]struct{}, maxLength int) {
	// Skip empty values to avoid noisy labels.
	if value == "" {
		return
	}

	// Skip labels that are not allowed.
	allowed := labelAllowed(key, allowlist, denylist)
	if !allowed {
		return
	}

	// Sanitize the label value before emitting.
	labels[key] = sanitizeLabelValue(value, maxLength)
}

// formatMetricLine renders a metric name with sorted labels.
func formatMetricLine(name string, labels map[string]string) string {
	// Return the bare metric name when no labels exist.
	if len(labels) == 0 {
		return name
	}

	// Sort label keys for stable output ordering.
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Build the label string.
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := labels[key]
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", key, value))
	}

	return fmt.Sprintf("%s{%s}", name, strings.Join(parts, ","))
}

// GenerateMetrics returns Prometheus metrics using the current time.
func GenerateMetrics(state health.State, config PromMetricsConfig) string {
	// Delegate to the time-aware implementation.
	return GenerateMetricsWithTime(state, config, time.Now())
}

// GenerateMetricsWithTime takes the state and returns it in Prometheus format.
func GenerateMetricsWithTime(state health.State, config PromMetricsConfig, now time.Time) string {
	// Prepare consistent label filters.
	allowlist := buildLabelSet(config.LabelAllowlist)
	denylist := buildLabelSet(config.LabelDenylist)

	// Start composing output.
	var output strings.Builder

	// Translate cluster health into a gauge value.
	healthStatus := "0"
	if state.OK {
		healthStatus = "1"
	}

	// Cluster status metric.
	output.WriteString("# HELP kuberhealthy_cluster_state Shows the status of the cluster\n")
	output.WriteString("# TYPE kuberhealthy_cluster_state gauge\n")
	output.WriteString(fmt.Sprintf("kuberhealthy_cluster_state %s\n", healthStatus))

	// Controller metrics.
	output.WriteString("# HELP kuberhealthy_controller_leader Shows whether this controller instance is the elected leader\n")
	output.WriteString("# TYPE kuberhealthy_controller_leader gauge\n")
	leaderValue := 0
	if state.Controller.IsLeader {
		leaderValue = 1
	}
	output.WriteString(fmt.Sprintf("kuberhealthy_controller_leader %d\n", leaderValue))

	output.WriteString("# HELP kuberhealthy_scheduler_loop_duration_seconds Shows the duration of the last scheduling loop\n")
	output.WriteString("# TYPE kuberhealthy_scheduler_loop_duration_seconds gauge\n")
	output.WriteString(fmt.Sprintf("kuberhealthy_scheduler_loop_duration_seconds %f\n", state.Controller.SchedulerLoopDurationSeconds))

	output.WriteString("# HELP kuberhealthy_scheduler_due_checks Shows the number of checks that were due in the last scheduling loop\n")
	output.WriteString("# TYPE kuberhealthy_scheduler_due_checks gauge\n")
	output.WriteString(fmt.Sprintf("kuberhealthy_scheduler_due_checks %d\n", state.Controller.SchedulerDueChecks))

	output.WriteString("# HELP kuberhealthy_reaper_last_sweep_duration_seconds Shows the duration of the last reaper sweep\n")
	output.WriteString("# TYPE kuberhealthy_reaper_last_sweep_duration_seconds gauge\n")
	output.WriteString(fmt.Sprintf("kuberhealthy_reaper_last_sweep_duration_seconds %f\n", state.Controller.ReaperLastSweepDurationSeconds))

	output.WriteString("# HELP kuberhealthy_reaper_deleted_pods_total Shows the total pods deleted by the reaper\n")
	output.WriteString("# TYPE kuberhealthy_reaper_deleted_pods_total counter\n")
	reaperReasons := make([]string, 0, len(state.Controller.ReaperDeletedPodsTotalByReason))
	for reason := range state.Controller.ReaperDeletedPodsTotalByReason {
		reaperReasons = append(reaperReasons, reason)
	}
	sort.Strings(reaperReasons)
	for _, reason := range reaperReasons {
		value := state.Controller.ReaperDeletedPodsTotalByReason[reason]
		labels := map[string]string{"reason": reason}
		output.WriteString(fmt.Sprintf("%s %d\n", formatMetricLine("kuberhealthy_reaper_deleted_pods_total", labels), value))
	}

	// Prepare per-check metric collections.
	checkStateMetrics := make(map[string]string)
	checkDurationMetrics := make(map[string]string)
	checkFailureMetrics := make(map[string]string)
	checkSuccessMetrics := make(map[string]string)
	checkFailureTotalMetrics := make(map[string]string)
	checkSinceSuccessMetrics := make(map[string]string)

	// Histogram buckets for run durations (seconds).
	histogramBuckets := []float64{1, 5, 10, 30, 60, 120, 300}
	histogramCounts := make([]int, len(histogramBuckets))
	histogramCount := 0
	histogramSum := 0.0

	// Parse through all check details and append to metric sets.
	for checkName, detail := range state.CheckDetails {
		// Determine the check status for the gauge.
		checkStatus := "0"
		if detail.OK {
			checkStatus = "1"
		}

		// Base labels for all check metrics.
		labels := baseCheckLabels(checkName, detail.Namespace)

		// Include optional labels from check metadata.
		severity := ""
		if detail.Labels != nil {
			severity = detail.Labels["kuberhealthy.io/severity"]
		}
		addExtraLabel(labels, "severity", severity, allowlist, denylist, config.LabelValueMaxLength)

		category := ""
		if detail.Labels != nil {
			category = detail.Labels["kuberhealthy.io/category"]
		}
		addExtraLabel(labels, "category", category, allowlist, denylist, config.LabelValueMaxLength)

		addExtraLabel(labels, "run_uuid", detail.CurrentUUID, allowlist, denylist, config.LabelValueMaxLength)

		// Build the status metric with status and error labels.
		statusLabels := copyLabels(labels)
		statusLabels["status"] = checkStatus
		if !config.SuppressErrorLabel {
			errorValue := errorLabelValue(detail.Errors, config.ErrorLabelMaxLength)
			statusLabels["error"] = errorValue
		}
		checkStateMetrics[formatMetricLine("kuberhealthy_check", statusLabels)] = checkStatus

		// Check duration gauge.
		durationLabels := copyLabels(labels)
		durationSeconds := detail.LastRunDuration.Seconds()
		if durationSeconds < 0 {
			durationSeconds = 0
		}
		checkDurationMetrics[formatMetricLine("kuberhealthy_check_duration_seconds", durationLabels)] = fmt.Sprintf("%f", durationSeconds)

		// Consecutive failure gauge.
		failureLabels := copyLabels(labels)
		checkFailureMetrics[formatMetricLine("kuberhealthy_check_consecutive_failures", failureLabels)] = fmt.Sprintf("%d", detail.ConsecutiveFailures)

		// Success/failure counters.
		successLabels := copyLabels(labels)
		checkSuccessMetrics[formatMetricLine("kuberhealthy_check_success_total", successLabels)] = fmt.Sprintf("%d", detail.SuccessCount)
		failureTotalLabels := copyLabels(labels)
		checkFailureTotalMetrics[formatMetricLine("kuberhealthy_check_failure_total", failureTotalLabels)] = fmt.Sprintf("%d", detail.FailureCount)

		// Time since last success gauge.
		secondsSinceSuccess := -1.0
		if detail.LastOKUnix > 0 {
			secondsSinceSuccess = now.Sub(time.Unix(detail.LastOKUnix, 0)).Seconds()
		}
		checkSinceSuccessMetrics[formatMetricLine("kuberhealthy_check_seconds_since_success", labels)] = fmt.Sprintf("%f", secondsSinceSuccess)

		// Update histogram stats with this check's duration.
		histogramCount++
		histogramSum += durationSeconds
		for i, bucket := range histogramBuckets {
			if durationSeconds <= bucket {
				histogramCounts[i]++
			}
		}
	}

	// Emit check status metrics.
	output.WriteString("# HELP kuberhealthy_check Shows the status of a Kuberhealthy check\n")
	output.WriteString("# TYPE kuberhealthy_check gauge\n")
	for _, key := range sortedMetricKeys(checkStateMetrics) {
		output.WriteString(fmt.Sprintf("%s %s\n", key, checkStateMetrics[key]))
	}

	// Emit check duration metrics.
	output.WriteString("# HELP kuberhealthy_check_duration_seconds Shows the check run duration of a Kuberhealthy check\n")
	output.WriteString("# TYPE kuberhealthy_check_duration_seconds gauge\n")
	for _, key := range sortedMetricKeys(checkDurationMetrics) {
		output.WriteString(fmt.Sprintf("%s %s\n", key, checkDurationMetrics[key]))
	}

	// Emit consecutive failure metrics.
	output.WriteString("# HELP kuberhealthy_check_consecutive_failures Shows the number of consecutive failures for a Kuberhealthy check\n")
	output.WriteString("# TYPE kuberhealthy_check_consecutive_failures gauge\n")
	for _, key := range sortedMetricKeys(checkFailureMetrics) {
		output.WriteString(fmt.Sprintf("%s %s\n", key, checkFailureMetrics[key]))
	}

	// Emit success counters.
	output.WriteString("# HELP kuberhealthy_check_success_total Shows the total number of successful runs for a check\n")
	output.WriteString("# TYPE kuberhealthy_check_success_total counter\n")
	for _, key := range sortedMetricKeys(checkSuccessMetrics) {
		output.WriteString(fmt.Sprintf("%s %s\n", key, checkSuccessMetrics[key]))
	}

	// Emit failure counters.
	output.WriteString("# HELP kuberhealthy_check_failure_total Shows the total number of failed runs for a check\n")
	output.WriteString("# TYPE kuberhealthy_check_failure_total counter\n")
	for _, key := range sortedMetricKeys(checkFailureTotalMetrics) {
		output.WriteString(fmt.Sprintf("%s %s\n", key, checkFailureTotalMetrics[key]))
	}

	// Emit time-since-success gauges.
	output.WriteString("# HELP kuberhealthy_check_seconds_since_success Shows time since the last OK report for a check\n")
	output.WriteString("# TYPE kuberhealthy_check_seconds_since_success gauge\n")
	for _, key := range sortedMetricKeys(checkSinceSuccessMetrics) {
		output.WriteString(fmt.Sprintf("%s %s\n", key, checkSinceSuccessMetrics[key]))
	}

	// Emit run duration histogram.
	output.WriteString("# HELP kuberhealthy_check_run_duration_seconds Bucketed distribution of check run durations\n")
	output.WriteString("# TYPE kuberhealthy_check_run_duration_seconds histogram\n")
	for i, bucket := range histogramBuckets {
		labels := map[string]string{"le": fmt.Sprintf("%g", bucket)}
		value := histogramCounts[i]
		output.WriteString(fmt.Sprintf("%s %d\n", formatMetricLine("kuberhealthy_check_run_duration_seconds_bucket", labels), value))
	}
	infLabels := map[string]string{"le": "+Inf"}
	output.WriteString(fmt.Sprintf("%s %d\n", formatMetricLine("kuberhealthy_check_run_duration_seconds_bucket", infLabels), histogramCount))
	output.WriteString(fmt.Sprintf("kuberhealthy_check_run_duration_seconds_sum %f\n", histogramSum))
	output.WriteString(fmt.Sprintf("kuberhealthy_check_run_duration_seconds_count %d\n", histogramCount))

	return output.String()
}

// sortedMetricKeys returns metric keys in deterministic order.
func sortedMetricKeys(metrics map[string]string) []string {
	// Return empty when there are no metrics.
	if len(metrics) == 0 {
		return nil
	}

	// Build and sort the keys.
	keys := make([]string, 0, len(metrics))
	for key := range metrics {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}
