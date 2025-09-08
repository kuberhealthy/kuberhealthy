package main

import (
	"testing"
	"time"
)

// TestLoadFromEnv populates configuration fields from environment variables.
func TestLoadFromEnv(t *testing.T) {
	t.Setenv("KH_LISTEN_ADDRESS", "127.0.0.1:9000")
	t.Setenv("KH_LOG_LEVEL", "debug")
	t.Setenv("KH_MAX_JOB_AGE", "30m")
	t.Setenv("KH_MAX_CHECK_POD_AGE", "15m")
	t.Setenv("KH_MAX_COMPLETED_POD_COUNT", "5")
	t.Setenv("KH_MAX_ERROR_POD_COUNT", "2")
	t.Setenv("KH_ERROR_POD_RETENTION_DAYS", "3")
	t.Setenv("KH_PROM_SUPPRESS_ERROR_LABEL", "true")
	t.Setenv("KH_PROM_ERROR_LABEL_MAX_LENGTH", "20")
	t.Setenv("KH_TARGET_NAMESPACE", "testing")
	t.Setenv("KH_DEFAULT_RUN_INTERVAL", "3m")
	t.Setenv("KH_CHECK_REPORT_HOSTNAME", "example.com")
	t.Setenv("KH_TERMINATION_GRACE_PERIOD", "30s")
	t.Setenv("KH_DEFAULT_CHECK_TIMEOUT", "1m")
	t.Setenv("KH_DEFAULT_NAMESPACE", "fallback")
	t.Setenv("POD_NAMESPACE", "podns")
	t.Setenv("KH_SERVICE_NAME", "svcname")

	cfg := New()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}

	if cfg.ListenAddress != "127.0.0.1:9000" {
		t.Errorf("ListenAddress parsed incorrectly: %s", cfg.ListenAddress)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel parsed incorrectly: %s", cfg.LogLevel)
	}
	if cfg.MaxKHJobAge != 30*time.Minute {
		t.Errorf("MaxKHJobAge parsed incorrectly: %v", cfg.MaxKHJobAge)
	}
	if cfg.MaxCheckPodAge != 15*time.Minute {
		t.Errorf("MaxCheckPodAge parsed incorrectly: %v", cfg.MaxCheckPodAge)
	}
	if cfg.MaxCompletedPodCount != 5 {
		t.Errorf("MaxCompletedPodCount parsed incorrectly: %d", cfg.MaxCompletedPodCount)
	}
	if cfg.MaxErrorPodCount != 2 {
		t.Errorf("MaxErrorPodCount parsed incorrectly: %d", cfg.MaxErrorPodCount)
	}
	if cfg.ErrorPodRetentionDays != 3 {
		t.Errorf("ErrorPodRetentionDays parsed incorrectly: %d", cfg.ErrorPodRetentionDays)
	}
	if !cfg.PromMetricsConfig.SuppressErrorLabel {
		t.Errorf("PromMetricsConfig.SuppressErrorLabel parsed incorrectly")
	}
	if cfg.PromMetricsConfig.ErrorLabelMaxLength != 20 {
		t.Errorf("PromMetricsConfig.ErrorLabelMaxLength parsed incorrectly: %d", cfg.PromMetricsConfig.ErrorLabelMaxLength)
	}
	if cfg.TargetNamespace != "testing" {
		t.Errorf("TargetNamespace parsed incorrectly: %s", cfg.TargetNamespace)
	}
	if cfg.DefaultRunInterval != 3*time.Minute {
		t.Errorf("DefaultRunInterval parsed incorrectly: %v", cfg.DefaultRunInterval)
	}
	if cfg.ReportingURL() != "http://example.com/check" {
		t.Errorf("ReportingURL parsed incorrectly: %s", cfg.ReportingURL())
	}
	if cfg.Namespace != "podns" {
		t.Errorf("Namespace parsed incorrectly: %s", cfg.Namespace)
	}
	if cfg.ServiceName != "svcname" {
		t.Errorf("ServiceName parsed incorrectly: %s", cfg.ServiceName)
	}
	if cfg.TerminationGracePeriod != 30*time.Second {
		t.Errorf("TerminationGracePeriodSeconds parsed incorrectly: %v", cfg.TerminationGracePeriod)
	}
	if cfg.DefaultCheckTimeout != time.Minute {
		t.Errorf("DefaultCheckTimeout parsed incorrectly: %v", cfg.DefaultCheckTimeout)
	}
	if cfg.DefaultNamespace != "fallback" {
		t.Errorf("DefaultNamespace parsed incorrectly: %s", cfg.DefaultNamespace)
	}
}

// TestLoadFromEnvInvalid returns an error when a duration environment variable is malformed.
func TestLoadFromEnvInvalid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping invalid config test in short mode")
	}
	t.Setenv("KH_MAX_JOB_AGE", "not-a-duration")
	cfg := New()
	if err := cfg.LoadFromEnv(); err == nil {
		t.Fatalf("expected error for invalid duration")
	}
}

// TestReportingURLFromServiceAndNamespace constructs the reporting URL when service name and namespace are provided.
func TestReportingURLFromServiceAndNamespace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping reporting URL test in short mode")
	}
	t.Setenv("POD_NAMESPACE", "ns1")
	t.Setenv("KH_SERVICE_NAME", "svc1")

	cfg := New()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}

	expected := "http://svc1.ns1.svc.cluster.local/check"
	if cfg.ReportingURL() != expected {
		t.Errorf("ReportingURL parsed incorrectly: %s", cfg.ReportingURL())
	}
}

func TestLoadTLSFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TLS file test in short mode")
	}
	t.Setenv("KH_TLS_CERT_FILE", "/cert")
	t.Setenv("KH_TLS_KEY_FILE", "/key")
	cfg := New()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
	if cfg.TLSCertFile != "/cert" {
		t.Errorf("TLSCertFile parsed incorrectly: %s", cfg.TLSCertFile)
	}
	if cfg.TLSKeyFile != "/key" {
		t.Errorf("TLSKeyFile parsed incorrectly: %s", cfg.TLSKeyFile)
	}
}

// TestTargetNamespaceDefaultsBlank ensures that when KH_TARGET_NAMESPACE is unset the target namespace is empty
func TestTargetNamespaceDefaultsBlank(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping target namespace default test in short mode")
	}
	t.Setenv("POD_NAMESPACE", "podns")
	cfg := New()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
	if cfg.TargetNamespace != "" {
		t.Errorf("expected blank TargetNamespace, got %s", cfg.TargetNamespace)
	}
}
