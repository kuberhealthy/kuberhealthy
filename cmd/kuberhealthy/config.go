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
	ListenAddressTLS            string
	ListenAddress               string
	LogLevel                    string
	MaxKHJobAge                 time.Duration
	MaxCheckPodAge              time.Duration
	MaxCompletedPodCount        int
	MaxErrorPodCount            int
	ErrorPodRetentionTime       time.Duration
	PromMetricsConfig           metrics.PromMetricsConfig
	TargetNamespace             string
	DefaultRunInterval          time.Duration
	checkReportBaseURL          string // base URL checks will report to (protocol, host, port; no path)
	TerminationGracePeriod      time.Duration
	DefaultCheckTimeout         time.Duration
	DefaultNamespace            string
	Namespace                   string // the namespace kh is running in
	TLSCertFile                 string
	TLSKeyFile                  string
	LeaderElectionEnabled       bool
	LeaderElectionNamespace     string
	LeaderElectionName          string
	LeaderElectionLeaseDuration time.Duration
	LeaderElectionRenewDeadline time.Duration
	LeaderElectionRetryPeriod   time.Duration
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
		MaxCompletedPodCount:   1,
		MaxErrorPodCount:       2,
		ErrorPodRetentionTime:  36 * time.Hour,
		PromMetricsConfig: metrics.PromMetricsConfig{
			LabelAllowlist:      []string{"severity", "category"},
			LabelValueMaxLength: 256,
		},
		LeaderElectionEnabled:       false,
		LeaderElectionNamespace:     ns,
		LeaderElectionName:          "kuberhealthy-controller",
		LeaderElectionLeaseDuration: time.Second * 15,
		LeaderElectionRenewDeadline: time.Second * 10,
		LeaderElectionRetryPeriod:   time.Second * 2,
	}
}

// parseLabelList splits comma-separated values into a de-duplicated list.
func parseLabelList(rawValue string) []string {
	// Trim whitespace to avoid accidental blank entries.
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return nil
	}

	// Split into fields and normalize whitespace.
	parts := strings.Split(trimmed, ",")
	seen := make(map[string]struct{})
	labels := make([]string, 0, len(parts))

	// Collect unique label keys.
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		_, exists := seen[value]
		if exists {
			continue
		}
		seen[value] = struct{}{}
		labels = append(labels, value)
	}

	return labels
}

// LoadFromEnv populates the config from environment variables.
func (c *Config) LoadFromEnv() error {
	// override listen address when an alternate value is provided
	listenAddress := os.Getenv("KH_LISTEN_ADDRESS")
	if listenAddress != "" {
		c.ListenAddress = listenAddress
	}

	// override the TLS listener address from the environment
	listenAddressTLS := os.Getenv("KH_LISTEN_ADDRESS_TLS")
	if listenAddressTLS != "" {
		c.ListenAddressTLS = listenAddressTLS
	}

	// allow configuring the global log level with KH_LOG_LEVEL
	logLevel := os.Getenv("KH_LOG_LEVEL")
	if logLevel != "" {
		c.LogLevel = logLevel
	}

	// parse the max job age duration override
	maxJobAge := os.Getenv("KH_MAX_JOB_AGE")
	if maxJobAge != "" {
		parsedDuration, err := time.ParseDuration(maxJobAge)
		if err != nil {
			return fmt.Errorf("invalid KH_MAX_JOB_AGE: %w", err)
		}
		c.MaxKHJobAge = parsedDuration
	}

	// parse the max check pod age duration override
	maxCheckPodAge := os.Getenv("KH_MAX_CHECK_POD_AGE")
	if maxCheckPodAge != "" {
		parsedDuration, err := time.ParseDuration(maxCheckPodAge)
		if err != nil {
			return fmt.Errorf("invalid KH_MAX_CHECK_POD_AGE: %w", err)
		}
		c.MaxCheckPodAge = parsedDuration
	}

	// parse the completed pod retention limit override
	maxCompletedPodCount := os.Getenv("KH_MAX_COMPLETED_POD_COUNT")
	if maxCompletedPodCount != "" {
		parsedCount, err := strconv.Atoi(maxCompletedPodCount)
		if err != nil {
			return fmt.Errorf("invalid KH_MAX_COMPLETED_POD_COUNT: %w", err)
		}
		if parsedCount < 0 {
			return fmt.Errorf("invalid KH_MAX_COMPLETED_POD_COUNT: value must be non-negative")
		}
		c.MaxCompletedPodCount = parsedCount
	}

	// parse the error pod retention limit override
	maxErrorPodCount := os.Getenv("KH_MAX_ERROR_POD_COUNT")
	if maxErrorPodCount != "" {
		parsedCount, err := strconv.Atoi(maxErrorPodCount)
		if err != nil {
			return fmt.Errorf("invalid KH_MAX_ERROR_POD_COUNT: %w", err)
		}
		if parsedCount < 0 {
			return fmt.Errorf("invalid KH_MAX_ERROR_POD_COUNT: value must be non-negative")
		}
		c.MaxErrorPodCount = parsedCount
	}

	// parse the error pod retention window override
	errorPodRetentionTime := os.Getenv("KH_ERROR_POD_RETENTION_TIME")
	if errorPodRetentionTime != "" {
		parsedDuration, err := time.ParseDuration(errorPodRetentionTime)
		if err != nil {
			return fmt.Errorf("invalid KH_ERROR_POD_RETENTION_TIME: %w", err)
		}
		if parsedDuration < 0 {
			return fmt.Errorf("invalid KH_ERROR_POD_RETENTION_TIME: value must be non-negative")
		}
		c.ErrorPodRetentionTime = parsedDuration
	}

	// parse the metrics error suppression toggle
	suppressErrorLabel := os.Getenv("KH_PROM_SUPPRESS_ERROR_LABEL")
	if suppressErrorLabel != "" {
		parsedBool, err := strconv.ParseBool(suppressErrorLabel)
		if err != nil {
			return fmt.Errorf("invalid KH_PROM_SUPPRESS_ERROR_LABEL: %w", err)
		}
		c.PromMetricsConfig.SuppressErrorLabel = parsedBool
	}

	// parse the metrics error label truncation override
	errorLabelMaxLength := os.Getenv("KH_PROM_ERROR_LABEL_MAX_LENGTH")
	if errorLabelMaxLength != "" {
		parsedLength, err := strconv.Atoi(errorLabelMaxLength)
		if err != nil {
			return fmt.Errorf("invalid KH_PROM_ERROR_LABEL_MAX_LENGTH: %w", err)
		}
		if parsedLength < 0 {
			return fmt.Errorf("invalid KH_PROM_ERROR_LABEL_MAX_LENGTH: value must be non-negative")
		}
		c.PromMetricsConfig.ErrorLabelMaxLength = parsedLength
	}

	// parse the metrics label allowlist override
	labelAllowlist := os.Getenv("KH_PROM_LABEL_ALLOWLIST")
	if labelAllowlist != "" {
		c.PromMetricsConfig.LabelAllowlist = parseLabelList(labelAllowlist)
	}

	// parse the metrics label denylist override
	labelDenylist := os.Getenv("KH_PROM_LABEL_DENYLIST")
	if labelDenylist != "" {
		c.PromMetricsConfig.LabelDenylist = parseLabelList(labelDenylist)
	}

	// parse the metrics label value truncation override
	labelValueMaxLength := os.Getenv("KH_PROM_LABEL_VALUE_MAX_LENGTH")
	if labelValueMaxLength != "" {
		parsedLength, err := strconv.Atoi(labelValueMaxLength)
		if err != nil {
			return fmt.Errorf("invalid KH_PROM_LABEL_VALUE_MAX_LENGTH: %w", err)
		}
		if parsedLength < 0 {
			return fmt.Errorf("invalid KH_PROM_LABEL_VALUE_MAX_LENGTH: value must be non-negative")
		}
		c.PromMetricsConfig.LabelValueMaxLength = parsedLength
	}

	// update the target namespace for checks when provided
	targetNamespace := os.Getenv("KH_TARGET_NAMESPACE")
	if targetNamespace != "" {
		c.TargetNamespace = targetNamespace
	}

	// capture the running namespace for service discovery
	podNamespace := os.Getenv("POD_NAMESPACE")
	if podNamespace != "" {
		c.Namespace = podNamespace
	}

	// parse the default run interval override
	defaultRunInterval := os.Getenv("KH_DEFAULT_RUN_INTERVAL")
	if defaultRunInterval != "" {
		parsedDuration, err := time.ParseDuration(defaultRunInterval)
		if err != nil {
			return fmt.Errorf("invalid KH_DEFAULT_RUN_INTERVAL: %w", err)
		}
		c.DefaultRunInterval = parsedDuration
	}

	// parse and normalize the check report endpoint override
	checkReportURL := os.Getenv("KH_CHECK_REPORT_URL")
	if checkReportURL == "" {
		c.checkReportBaseURL = fmt.Sprintf("http://kuberhealthy.%s.svc.cluster.local:8080", c.Namespace)
		log.Warnln("KH_CHECK_REPORT_URL environment variable not set. Using", c.checkReportBaseURL)
	}
	if checkReportURL != "" {
		trimmed := strings.TrimSpace(checkReportURL)
		trimmed = strings.TrimRight(trimmed, "/")
		if strings.HasSuffix(trimmed, "/check") {
			return fmt.Errorf("invalid KH_CHECK_REPORT_URL: do not include '/check' in %q", checkReportURL)
		}
		if trimmed == "" {
			return fmt.Errorf("invalid KH_CHECK_REPORT_URL: %q", checkReportURL)
		}
		c.checkReportBaseURL = trimmed
	}

	// parse the termination grace period override
	terminationGracePeriod := os.Getenv("KH_TERMINATION_GRACE_PERIOD")
	if terminationGracePeriod != "" {
		parsedDuration, err := time.ParseDuration(terminationGracePeriod)
		if err != nil {
			return fmt.Errorf("invalid KH_TERMINATION_GRACE_PERIOD: %w", err)
		}
		c.TerminationGracePeriod = parsedDuration
	}

	// parse the default check timeout override
	defaultCheckTimeout := os.Getenv("KH_DEFAULT_CHECK_TIMEOUT")
	if defaultCheckTimeout != "" {
		parsedDuration, err := time.ParseDuration(defaultCheckTimeout)
		if err != nil {
			return fmt.Errorf("invalid KH_DEFAULT_CHECK_TIMEOUT: %w", err)
		}
		c.DefaultCheckTimeout = parsedDuration
	}

	// override the namespace checks default to when none is specified
	defaultNamespace := os.Getenv("KH_DEFAULT_NAMESPACE")
	if defaultNamespace != "" {
		c.DefaultNamespace = defaultNamespace
	}

	// capture TLS certificate paths when supplied
	tlsCertFile := os.Getenv("KH_TLS_CERT_FILE")
	if tlsCertFile != "" {
		c.TLSCertFile = tlsCertFile
	}

	tlsKeyFile := os.Getenv("KH_TLS_KEY_FILE")
	if tlsKeyFile != "" {
		c.TLSKeyFile = tlsKeyFile
	}

	// parse leader election toggle
	leaderElectionEnabled := os.Getenv("KH_LEADER_ELECTION_ENABLED")
	if leaderElectionEnabled != "" {
		parsedBool, err := strconv.ParseBool(leaderElectionEnabled)
		if err != nil {
			return fmt.Errorf("invalid KH_LEADER_ELECTION_ENABLED: %w", err)
		}
		c.LeaderElectionEnabled = parsedBool
	}

	// parse leader election lease name override
	leaderElectionName := os.Getenv("KH_LEADER_ELECTION_NAME")
	if leaderElectionName != "" {
		c.LeaderElectionName = leaderElectionName
	}

	// parse leader election namespace override
	leaderElectionNamespace := os.Getenv("KH_LEADER_ELECTION_NAMESPACE")
	if leaderElectionNamespace != "" {
		c.LeaderElectionNamespace = leaderElectionNamespace
	}
	if leaderElectionNamespace == "" {
		c.LeaderElectionNamespace = c.Namespace
	}

	// parse leader election lease duration override
	leaderElectionLeaseDuration := os.Getenv("KH_LEADER_ELECTION_LEASE_DURATION")
	if leaderElectionLeaseDuration != "" {
		parsedDuration, err := time.ParseDuration(leaderElectionLeaseDuration)
		if err != nil {
			return fmt.Errorf("invalid KH_LEADER_ELECTION_LEASE_DURATION: %w", err)
		}
		c.LeaderElectionLeaseDuration = parsedDuration
	}

	// parse leader election renew deadline override
	leaderElectionRenewDeadline := os.Getenv("KH_LEADER_ELECTION_RENEW_DEADLINE")
	if leaderElectionRenewDeadline != "" {
		parsedDuration, err := time.ParseDuration(leaderElectionRenewDeadline)
		if err != nil {
			return fmt.Errorf("invalid KH_LEADER_ELECTION_RENEW_DEADLINE: %w", err)
		}
		c.LeaderElectionRenewDeadline = parsedDuration
	}

	// parse leader election retry period override
	leaderElectionRetryPeriod := os.Getenv("KH_LEADER_ELECTION_RETRY_PERIOD")
	if leaderElectionRetryPeriod != "" {
		parsedDuration, err := time.ParseDuration(leaderElectionRetryPeriod)
		if err != nil {
			return fmt.Errorf("invalid KH_LEADER_ELECTION_RETRY_PERIOD: %w", err)
		}
		c.LeaderElectionRetryPeriod = parsedDuration
	}

	// validate leader election timing constraints
	err := c.validateLeaderElectionConfig()
	if err != nil {
		return err
	}

	return nil
}

// validateLeaderElectionConfig enforces timing relationships for leader election durations.
func (c *Config) validateLeaderElectionConfig() error {
	if c.LeaderElectionLeaseDuration <= 0 {
		return fmt.Errorf("invalid leader election lease duration: %s", c.LeaderElectionLeaseDuration)
	}
	if c.LeaderElectionRenewDeadline <= 0 {
		return fmt.Errorf("invalid leader election renew deadline: %s", c.LeaderElectionRenewDeadline)
	}
	if c.LeaderElectionRetryPeriod <= 0 {
		return fmt.Errorf("invalid leader election retry period: %s", c.LeaderElectionRetryPeriod)
	}
	if c.LeaderElectionRenewDeadline >= c.LeaderElectionLeaseDuration {
		return fmt.Errorf("invalid leader election renew deadline: must be less than lease duration")
	}
	if c.LeaderElectionRetryPeriod >= c.LeaderElectionRenewDeadline {
		return fmt.Errorf("invalid leader election retry period: must be less than renew deadline")
	}
	return nil
}

// ReportingURL returns the full URL for check reporting
func (c *Config) ReportingURL() string {
	base := strings.TrimRight(c.checkReportBaseURL, "/")
	return base + "/check"
}
