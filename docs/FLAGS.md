Available flags and environment variables for use in Kuberhealthy

# Flags

| Flag      | Description                           | Optional | Default |
| --------- | ------------------------------------- | -------- | ------- |
| `--debug` | Bool to enable/disable debug logging. | Yes      | `False` |

# Environment Variables

| Variable | Description | Default |
| -------- | ----------- | ------- |
| `KH_LISTEN_ADDRESS` | Address for the web server | `:8080` |
| `KH_LOG_LEVEL` | Log level (trace, debug, info, warn, error, fatal, panic) | `info` |
| `KH_MAX_JOB_AGE` | Maximum age for check jobs before cleanup | unset |
| `KH_MAX_CHECK_POD_AGE` | Maximum age for check pods before cleanup | unset |
| `KH_MAX_COMPLETED_POD_COUNT` | Number of completed pods to retain | `0` |
| `KH_MAX_ERROR_POD_COUNT` | Number of error pods to retain | `0` |
| `KH_PROM_SUPPRESS_ERROR_LABEL` | Omit error label in Prometheus metrics | `false` |
| `KH_PROM_ERROR_LABEL_MAX_LENGTH` | Maximum length for Prometheus error label | `0` |
| `KH_TARGET_NAMESPACE` | Namespace Kuberhealthy operates in (blank for all) | <pod namespace> |
| `KH_DEFAULT_RUN_INTERVAL` | Default check run interval | `10m` |
| `KH_CHECK_REPORT_HOSTNAME` | Hostname used for check reports | `kuberhealthy.kuberhealthy.svc.cluster.local` |
| `KH_TERMINATION_GRACE_PERIOD` | Shutdown grace period | `5m` |
| `KH_DEFAULT_CHECK_TIMEOUT` | Default timeout for checks | `5m` |
| `KH_DEBUG_MODE` | Enable debug mode | `false` |
| `KH_DEFAULT_NAMESPACE` | Fallback namespace if detection fails | `kuberhealthy` |
