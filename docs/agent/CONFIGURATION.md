# Configuration

Kuberhealthy is configured via environment variables. There are no command-line flags. The defaults below match the values set in `deploy/kustomize/base/deployment.yaml` and `cmd/kuberhealthy/config.go`.

## Controller environment variables

| Variable | Description | Default |
|---|---|---|
| `KH_LISTEN_ADDRESS` | HTTP listen address for the main web server | `:8080` |
| `KH_LISTEN_ADDRESS_TLS` | HTTPS listen address when TLS is enabled | `:443` |
| `KH_LOG_LEVEL` | Log level (`trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`) | `info` |
| `KH_MAX_JOB_AGE` | Legacy setting for job cleanup (unused in v3). Go duration syntax | unset |
| `KH_MAX_CHECK_POD_AGE` | Maximum age for check pods before cleanup, regardless of phase. Empty disables age-based cleanup | unset |
| `KH_MAX_COMPLETED_POD_COUNT` | Maximum number of completed check pods to retain | `1` |
| `KH_MAX_ERROR_POD_COUNT` | Number of failed check pods to retain for debugging | `2` |
| `KH_ERROR_POD_RETENTION_TIME` | Duration to retain failed check pods when count-based retention is not reached | `36h` |
| `KH_PROM_SUPPRESS_ERROR_LABEL` | When `true`, omit the error label on Prometheus metrics | `false` |
| `KH_PROM_ERROR_LABEL_MAX_LENGTH` | Maximum length of the Prometheus error label; `0` disables truncation | `0` |
| `KH_PROM_LABEL_ALLOWLIST` | Comma-separated list of extra label keys to include (e.g. `severity,category`) | `severity,category` |
| `KH_PROM_LABEL_DENYLIST` | Comma-separated list of extra label keys to exclude from metrics | unset |
| `KH_PROM_LABEL_VALUE_MAX_LENGTH` | Maximum length of extra label values; `0` disables truncation | `256` |
| `KH_TARGET_NAMESPACE` | Namespace to watch for `HealthCheck` resources; empty means all namespaces | `` |
| `KH_DEFAULT_RUN_INTERVAL` | Default run interval for checks that omit `spec.runInterval` | `10m` |
| `KH_CHECK_REPORT_URL` | Base URL used by checker pods to report results; `/check` is appended automatically. Do not include `/check` | `http://kuberhealthy.<namespace>.svc.cluster.local` |
| `KH_TERMINATION_GRACE_PERIOD` | Time to wait for clean shutdown before forced exit | `5m` |
| `KH_DEFAULT_CHECK_TIMEOUT` | Default timeout for checks that omit `spec.timeout` | `30s` |
| `KH_DEFAULT_NAMESPACE` | Fallback namespace when a check does not specify one | `kuberhealthy` |
| `POD_NAMESPACE` | Namespace of the running controller pod. Typically injected via the Downward API | `<pod namespace>` |
| `POD_NAME` | Name of the running controller pod. Used for logging; hostname used if unset | `<pod name>` |
| `KH_TLS_CERT_FILE` | Path to the TLS certificate file. If set with `KH_TLS_KEY_FILE`, an HTTPS listener is started | unset |
| `KH_TLS_KEY_FILE` | Path to the TLS private key file. If set with `KH_TLS_CERT_FILE`, an HTTPS listener is started | unset |
| `KH_LEADER_ELECTION_ENABLED` | Enable Lease-based leader election for check scheduling and reaping | `true` |
| `KH_LEADER_ELECTION_NAME` | Lease name used for leader election | `kuberhealthy-controller` |
| `KH_LEADER_ELECTION_NAMESPACE` | Namespace that stores the Lease object | `<POD_NAMESPACE>` |
| `KH_LEADER_ELECTION_LEASE_DURATION` | Lease duration for leader election | `15s` |
| `KH_LEADER_ELECTION_RENEW_DEADLINE` | Renewal deadline for leader election | `10s` |
| `KH_LEADER_ELECTION_RETRY_PERIOD` | Retry period for leader election | `2s` |

Leader election requires the controller service account to have `get`, `list`, `watch`, `create`, `update`, and `patch` access to `coordination.k8s.io` `leases` in the configured lease namespace.

## RBAC requirements

The controller ServiceAccount needs the ClusterRole permissions defined in `deploy/kustomize/base/clusterrole.yaml`:

- `kuberhealthy.github.io` `healthchecks` (including status and finalizers)
- Core `pods`, `pods/log`, and `events`
- `coordination.k8s.io` `leases` for leader election

## Checker pod environment variables

These are injected into every checker pod by the controller. They are not configured on the controller deployment.

| Variable | Description |
|---|---|
| `KH_REPORTING_URL` | Full URL for reporting check results |
| `KH_CHECK_RUN_DEADLINE` | Unix timestamp deadline derived from the check timeout |
| `KH_RUN_UUID` | Unique run identifier — send as the `kh-run-uuid` header |
| `KH_POD_NAMESPACE` | Namespace the checker pod is running in |

See [CHECK_CREATION.md](../CHECK_CREATION.md) for how to use them.
