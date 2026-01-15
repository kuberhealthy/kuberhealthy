# Configuration

Kuberhealthy is configured via environment variables. There are no command-line flags. The defaults below match the values set in `deploy/kustomize/base/deployment.yaml` and `cmd/kuberhealthy/config.go`.

## Controller Environment Variables

- `KH_LISTEN_ADDRESS`: HTTP listen address for the main web server. Default `:8080`.
- `KH_LISTEN_ADDRESS_TLS`: HTTPS listen address when TLS is enabled. Default `:443`.
- `KH_LOG_LEVEL`: Log level (`trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`). Default `info`.
- `KH_MAX_JOB_AGE`: Maximum age for check jobs before cleanup. Empty disables age-based cleanup. Uses Go duration syntax (for example `30m`).
- `KH_MAX_CHECK_POD_AGE`: Maximum age for check pods before cleanup, regardless of phase. Empty disables age-based cleanup. Uses Go duration syntax.
- `KH_MAX_COMPLETED_POD_COUNT`: Maximum number of completed check pods to retain. Default `1`.
- `KH_MAX_ERROR_POD_COUNT`: Number of failed check pods to retain for debugging. Default `2`.
- `KH_ERROR_POD_RETENTION_TIME`: Duration to retain failed check pods when count-based retention is not reached. Default `36h`.
- `KH_PROM_SUPPRESS_ERROR_LABEL`: When `true`, omit the error label on Prometheus metrics. Default `false`.
- `KH_PROM_ERROR_LABEL_MAX_LENGTH`: Maximum length of the Prometheus error label; `0` disables truncation. Default `0`.
- `KH_TARGET_NAMESPACE`: Namespace to watch for `HealthCheck` resources; empty means all namespaces.
- `KH_DEFAULT_RUN_INTERVAL`: Default run interval for checks that omit `spec.runInterval`. Default `10m`.
- `KH_CHECK_REPORT_URL`: Base URL used by checker pods to report results; `/check` is appended automatically and including `/check` will fail validation. Default `http://kuberhealthy.<namespace>.svc.cluster.local:8080`.
- `KH_TERMINATION_GRACE_PERIOD`: Time to wait for clean shutdown before forced exit. Default `5m`.
- `KH_DEFAULT_CHECK_TIMEOUT`: Default timeout for checks that omit `spec.timeout`. Default `30s`.
- `KH_DEFAULT_NAMESPACE`: Fallback namespace when a check does not specify one. Default `kuberhealthy`.
- `POD_NAMESPACE`: Namespace of the running controller pod. Used for service discovery and defaults. Typically injected via the Downward API.
- `POD_NAME`: Name of the running controller pod. Used for logging; if unset, the hostname is used.
- `KH_TLS_CERT_FILE`: Path to the TLS certificate file. If set with `KH_TLS_KEY_FILE`, an HTTPS listener is started.
- `KH_TLS_KEY_FILE`: Path to the TLS private key file. If set with `KH_TLS_CERT_FILE`, an HTTPS listener is started.

## Checker Pod Environment Variables

These are injected into every checker pod and are required by the check clients:

- `KH_REPORTING_URL`: Full URL for reporting check results.
- `KH_CHECK_RUN_DEADLINE`: Unix timestamp deadline derived from the check timeout.
- `KH_RUN_UUID`: Unique run identifier that must be sent as `kh-run-uuid` header.
- `KH_POD_NAMESPACE`: Namespace the checker pod is running in.
