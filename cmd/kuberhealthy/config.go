package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/metrics"
	log "github.com/sirupsen/logrus"
)

// Config holds all configurable options
// Values are primarily sourced from environment variables.
type Config struct {
	ListenAddressTLS       string
	ListenAddress          string
	LogLevel               string
	MaxKHJobAge            time.Duration
	MaxCheckPodAge         time.Duration
	MaxCompletedPodCount   int
	MaxErrorPodCount       int
	ErrorPodRetentionDays  int
	PromMetricsConfig      metrics.PromMetricsConfig
	TargetNamespace        string
	DefaultRunInterval     time.Duration
	checkReportBaseURL     string // base URL checks will report to (protocol, host, port; no path)
	TerminationGracePeriod time.Duration
	DefaultCheckTimeout    time.Duration
	DefaultNamespace       string
	Namespace              string // the namespace kh is running in
	TLSCertFile            string
	TLSKeyFile             string
}

// New creates a Config populated with sane defaults.
func New() *Config {
	ns := os.Getenv("POD_NAMESPACE")
	if ns == "" {
		ns = GetMyNamespace("kuberhealthy")
	}

	return &Config{
		ListenAddress:    ":8080",
		ListenAddressTLS: ":443",
		LogLevel:         "info",
		// Default to the in-cluster service URL
		checkReportBaseURL:     fmt.Sprintf("http://kuberhealthy.%s.svc.cluster.local:8080", ns),
		DefaultRunInterval:     time.Minute * 10,
		TerminationGracePeriod: time.Minute * 5,
		DefaultCheckTimeout:    30 * time.Second,
		Namespace:              ns,
		MaxErrorPodCount:       5,
		ErrorPodRetentionDays:  4,
	}
}

// LoadFromEnv populates the config from environment variables.
func (c *Config) LoadFromEnv() error {
	if v := os.Getenv("KH_LISTEN_ADDRESS"); v != "" {
		c.ListenAddress = v
	}

	if v := os.Getenv("KH_LISTEN_ADDRESS_TLS"); v != "" {
		c.ListenAddressTLS = v
	}

	if v := os.Getenv("KH_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}

	if v := os.Getenv("KH_MAX_JOB_AGE"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid KH_MAX_JOB_AGE: %w", err)
		}
		c.MaxKHJobAge = d
	}

	if v := os.Getenv("KH_MAX_CHECK_POD_AGE"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid KH_MAX_CHECK_POD_AGE: %w", err)
		}
		c.MaxCheckPodAge = d
	}

	if v := os.Getenv("KH_MAX_COMPLETED_POD_COUNT"); v != "" {
		i, err := strconv.Atoi(v)
		if err != nil || i < 0 {
			return fmt.Errorf("invalid KH_MAX_COMPLETED_POD_COUNT: %v", err)
		}
		c.MaxCompletedPodCount = i
	}

	if v := os.Getenv("KH_MAX_ERROR_POD_COUNT"); v != "" {
		i, err := strconv.Atoi(v)
		if err != nil || i < 0 {
			return fmt.Errorf("invalid KH_MAX_ERROR_POD_COUNT: %v", err)
		}
		c.MaxErrorPodCount = i
	}

	if v := os.Getenv("KH_ERROR_POD_RETENTION_DAYS"); v != "" {
		i, err := strconv.Atoi(v)
		if err != nil || i < 0 {
			return fmt.Errorf("invalid KH_ERROR_POD_RETENTION_DAYS: %v", err)
		}
		c.ErrorPodRetentionDays = i
	}

	if v := os.Getenv("KH_PROM_SUPPRESS_ERROR_LABEL"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid KH_PROM_SUPPRESS_ERROR_LABEL: %w", err)
		}
		c.PromMetricsConfig.SuppressErrorLabel = b
	}

	if v := os.Getenv("KH_PROM_ERROR_LABEL_MAX_LENGTH"); v != "" {
		i, err := strconv.Atoi(v)
		if err != nil || i < 0 {
			return fmt.Errorf("invalid KH_PROM_ERROR_LABEL_MAX_LENGTH: %v", err)
		}
		c.PromMetricsConfig.ErrorLabelMaxLength = i
	}

	if v := os.Getenv("KH_TARGET_NAMESPACE"); v != "" {
		c.TargetNamespace = v
	}

	if v := os.Getenv("POD_NAMESPACE"); v != "" {
		c.Namespace = v
	}

	if v := os.Getenv("KH_DEFAULT_RUN_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid KH_DEFAULT_RUN_INTERVAL: %w", err)
		}
		c.DefaultRunInterval = d
	}

	if v := os.Getenv("KH_CHECK_REPORT_URL"); v != "" {
		trimmed := strings.TrimSpace(v)
		trimmed = strings.TrimRight(trimmed, "/")
		if strings.HasSuffix(trimmed, "/check") {
			trimmed = strings.TrimSuffix(trimmed, "/check")
			trimmed = strings.TrimRight(trimmed, "/")
			log.Warnln("KH_CHECK_REPORT_URL should not include '/check'. Trimming suffix for compatibility.")
		}
		if trimmed == "" {
			return fmt.Errorf("invalid KH_CHECK_REPORT_URL: %q", v)
		}
		c.checkReportBaseURL = trimmed
	} else {
		c.checkReportBaseURL = fmt.Sprintf("http://kuberhealthy.%s.svc.cluster.local:8080", c.Namespace)
		log.Warnln("KH_CHECK_REPORT_URL environment variable not set. Using", c.checkReportBaseURL)
	}

	if v := os.Getenv("KH_TERMINATION_GRACE_PERIOD"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid KH_TERMINATION_GRACE_PERIOD: %w", err)
		}
		c.TerminationGracePeriod = d
	}

	if v := os.Getenv("KH_DEFAULT_CHECK_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid KH_DEFAULT_CHECK_TIMEOUT: %w", err)
		}
		c.DefaultCheckTimeout = d
	}

	if v := os.Getenv("KH_DEFAULT_NAMESPACE"); v != "" {
		c.DefaultNamespace = v
	}

	if v := os.Getenv("KH_TLS_CERT_FILE"); v != "" {
		c.TLSCertFile = v
	}

	if v := os.Getenv("KH_TLS_KEY_FILE"); v != "" {
		c.TLSKeyFile = v
	}

	return nil
}

// ReportingURL returns the full URL for check reporting
func (c *Config) ReportingURL() string {
	base := strings.TrimRight(c.checkReportBaseURL, "/")
	return base + "/check"
}
