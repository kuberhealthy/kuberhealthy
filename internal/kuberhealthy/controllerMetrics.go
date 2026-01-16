package kuberhealthy

import (
	"time"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/health"
)

// controllerMetricsSnapshot stores controller-level metrics for Prometheus export.
type controllerMetricsSnapshot struct {
	SchedulerLoopDurationSeconds   float64
	SchedulerDueChecks             int
	ReaperLastSweepDurationSeconds float64
	ReaperDeletedPodsTotalByReason map[string]int64
}

// newControllerMetricsSnapshot creates a snapshot with initialized maps.
func newControllerMetricsSnapshot() controllerMetricsSnapshot {
	return controllerMetricsSnapshot{
		ReaperDeletedPodsTotalByReason: map[string]int64{},
	}
}

// recordSchedulerMetrics stores the latest scheduler loop metrics.
func (kh *Kuberhealthy) recordSchedulerMetrics(duration time.Duration, dueChecks int) {
	// Guard against nil receiver usage.
	if kh == nil {
		return
	}

	// Normalize negative durations.
	if duration < 0 {
		duration = 0
	}

	// Store the scheduler loop metrics under lock.
	kh.metricsMu.Lock()
	kh.metricsSnapshot.SchedulerLoopDurationSeconds = duration.Seconds()
	kh.metricsSnapshot.SchedulerDueChecks = dueChecks
	kh.metricsMu.Unlock()
}

// recordReaperMetrics stores the latest reaper sweep duration.
func (kh *Kuberhealthy) recordReaperMetrics(duration time.Duration) {
	// Guard against nil receiver usage.
	if kh == nil {
		return
	}

	// Normalize negative durations.
	if duration < 0 {
		duration = 0
	}

	// Store the reaper sweep duration under lock.
	kh.metricsMu.Lock()
	kh.metricsSnapshot.ReaperLastSweepDurationSeconds = duration.Seconds()
	kh.metricsMu.Unlock()
}

// incrementReaperDelete records a deletion event for the provided reason.
func (kh *Kuberhealthy) incrementReaperDelete(reason string) {
	// Guard against nil receiver usage.
	if kh == nil {
		return
	}

	// Ignore empty reasons to keep metrics clean.
	if reason == "" {
		return
	}

	// Update the running totals under lock.
	kh.metricsMu.Lock()
	kh.metricsSnapshot.ReaperDeletedPodsTotalByReason[reason]++
	kh.metricsMu.Unlock()
}

// MetricsSnapshot returns a copy of controller metrics for export.
func (kh *Kuberhealthy) MetricsSnapshot() health.ControllerMetrics {
	// Default snapshot when the controller is unavailable.
	if kh == nil {
		return health.NewControllerMetrics()
	}

	// Build a copy to avoid exposing internal maps.
	kh.metricsMu.Lock()
	copySnapshot := health.NewControllerMetrics()
	copySnapshot.IsLeader = kh.IsLeader()
	copySnapshot.SchedulerLoopDurationSeconds = kh.metricsSnapshot.SchedulerLoopDurationSeconds
	copySnapshot.SchedulerDueChecks = kh.metricsSnapshot.SchedulerDueChecks
	copySnapshot.ReaperLastSweepDurationSeconds = kh.metricsSnapshot.ReaperLastSweepDurationSeconds
	for reason, count := range kh.metricsSnapshot.ReaperDeletedPodsTotalByReason {
		copySnapshot.ReaperDeletedPodsTotalByReason[reason] = count
	}
	kh.metricsMu.Unlock()

	return copySnapshot
}
