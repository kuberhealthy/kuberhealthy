package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/health"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
)

// TestGenerateMetricsGolden verifies metrics output against a golden file for stability.
func TestGenerateMetricsGolden(t *testing.T) {
	// Build a fixed timestamp for deterministic metrics.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Configure label handling and truncation for the test.
	config := PromMetricsConfig{
		ErrorLabelMaxLength: 40,
		LabelAllowlist:      []string{"severity", "category", "run_uuid"},
		LabelValueMaxLength: 8,
	}

	// Build a representative state for metrics generation.
	state := health.NewState()
	state.OK = false
	state.Controller.IsLeader = true
	state.Controller.SchedulerLoopDurationSeconds = 1.25
	state.Controller.SchedulerDueChecks = 2
	state.Controller.ReaperLastSweepDurationSeconds = 0.75
	state.Controller.ReaperDeletedPodsTotalByReason["failed"] = 2
	state.Controller.ReaperDeletedPodsTotalByReason["timeout"] = 1

	// Add a successful check with metadata labels and a run UUID.
	daemonset := health.CheckDetail{HealthCheckStatus: khapi.HealthCheckStatus{}}
	daemonset.OK = true
	daemonset.Namespace = "kuberhealthy"
	daemonset.LastRunDuration = 4 * time.Second
	daemonset.ConsecutiveFailures = 0
	daemonset.SuccessCount = 3
	daemonset.FailureCount = 1
	daemonset.LastOKUnix = now.Add(-10 * time.Minute).Unix()
	daemonset.CurrentUUID = "run-1"
	daemonset.Labels = map[string]string{
		"kuberhealthy.io/severity": "critical",
		"kuberhealthy.io/category": "kube-infra",
	}
	state.CheckDetails["daemonset"] = daemonset

	// Add a failing check with errors and no recent success timestamp.
	deployment := health.CheckDetail{HealthCheckStatus: khapi.HealthCheckStatus{}}
	deployment.OK = false
	deployment.Namespace = "kuberhealthy"
	deployment.Errors = []string{"error one", "error two"}
	deployment.LastRunDuration = 12 * time.Second
	deployment.ConsecutiveFailures = 2
	deployment.SuccessCount = 1
	deployment.FailureCount = 4
	deployment.LastOKUnix = 0
	deployment.Labels = map[string]string{
		"kuberhealthy.io/severity": "warning",
		"kuberhealthy.io/category": "workloads",
	}
	state.CheckDetails["deployment"] = deployment

	// Generate metrics output and compare with the golden file.
	output := GenerateMetricsWithTime(state, config, now)
	goldenPath := filepath.Join("testdata", "metrics.golden")

	// Update the golden file when requested.
	updateGolden := os.Getenv("UPDATE_GOLDEN")
	if updateGolden == "1" {
		writeErr := os.WriteFile(goldenPath, []byte(output), 0o644)
		if writeErr != nil {
			t.Fatalf("failed to update golden file: %v", writeErr)
		}
		return
	}

	expected, readErr := os.ReadFile(goldenPath)
	if readErr != nil {
		t.Fatalf("failed to read golden file: %v", readErr)
	}

	if string(expected) != output {
		t.Fatalf("metrics output did not match golden file")
	}
}
