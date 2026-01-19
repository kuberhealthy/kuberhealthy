# Configuration

Kuberhealthy uses environment variables only. It does not accept command-line flags.

## Environment Variables

| Variable | Description | Default |
| -------- | ----------- | ------- |
| `KH_LISTEN_ADDRESS` | Address for the web server | `:8080` |
| `KH_LISTEN_ADDRESS_TLS` | Address for the HTTPS listener when TLS is enabled | `:443` |
| `KH_LOG_LEVEL` | Log level (`trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`) | `info` |
| `KH_MAX_JOB_AGE` | Legacy setting for job cleanup (unused in v3) | unset |
| `KH_MAX_CHECK_POD_AGE` | Maximum age for check pods before cleanup | unset |
| `KH_MAX_COMPLETED_POD_COUNT` | Number of completed pods to retain (`0` deletes all completed pods) | `1` |
| `KH_MAX_ERROR_POD_COUNT` | Number of errored pods to retain for debugging | `2` |
| `KH_ERROR_POD_RETENTION_TIME` | Duration to retain failed pods (Go duration syntax) | `36h` |
| `POD_NAMESPACE` | Namespace Kuberhealthy runs in | `<pod namespace>` |
| `KH_PROM_SUPPRESS_ERROR_LABEL` | Omit error label in Prometheus metrics | `false` |
| `KH_PROM_ERROR_LABEL_MAX_LENGTH` | Maximum length for Prometheus error label | `0` |
| `KH_PROM_LABEL_ALLOWLIST` | Comma-separated list of extra label keys to include (for example `severity,category`) | `severity,category` |
| `KH_PROM_LABEL_DENYLIST` | Comma-separated list of extra label keys to exclude | unset |
| `KH_PROM_LABEL_VALUE_MAX_LENGTH` | Maximum length for extra label values | `256` |
| `KH_TARGET_NAMESPACE` | Namespace Kuberhealthy operates in (blank for all) | `` |
| `KH_DEFAULT_RUN_INTERVAL` | Default check run interval | `10m` |
| `KH_CHECK_REPORT_URL` | Base URL used for check reports; `/check` is appended automatically | `http://kuberhealthy.<namespace>.svc.cluster.local` |
| `KH_TERMINATION_GRACE_PERIOD` | Shutdown grace period | `5m` |
| `KH_DEFAULT_CHECK_TIMEOUT` | Default timeout for checks | `30s` |
| `KH_DEFAULT_NAMESPACE` | Fallback namespace if detection fails | `kuberhealthy` |
| `KH_LEADER_ELECTION_ENABLED` | Enable Lease-based leader election for check scheduling | `false` |
| `KH_LEADER_ELECTION_NAME` | Lease name used for leader election | `kuberhealthy-controller` |
| `KH_LEADER_ELECTION_NAMESPACE` | Namespace that stores the Lease | `<pod namespace>` |
| `KH_LEADER_ELECTION_LEASE_DURATION` | Lease duration for leader election | `15s` |
| `KH_LEADER_ELECTION_RENEW_DEADLINE` | Renewal deadline for leader election | `10s` |
| `KH_LEADER_ELECTION_RETRY_PERIOD` | Retry period for leader election | `2s` |
| `POD_NAME` | Name of the running controller pod | `<pod name>` |
| `KH_TLS_CERT_FILE` | TLS certificate path for HTTPS listener | unset |
| `KH_TLS_KEY_FILE` | TLS private key path for HTTPS listener | unset |

Leader election requires the controller service account to have `get`, `list`, `watch`, `create`, `update`, and `patch` access to `coordination.k8s.io` `leases` in the Lease namespace.
