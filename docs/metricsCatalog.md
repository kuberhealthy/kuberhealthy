# Metrics Catalog (v3)

This catalog documents the Prometheus metrics exported by Kuberhealthy v3. Metric names and labels should be treated as stable within a major version. If changes are required, update this catalog and the release notes.

## Common labels

- `check`: HealthCheck name.
- `namespace`: HealthCheck namespace.
- `status`: `1` for OK, `0` for failure (only on `kuberhealthy_check`).
- `error`: Serialized error string (optional; suppressed when `KH_PROM_SUPPRESS_ERROR_LABEL=true`).
- `severity`, `category`, `run_uuid`: Optional extra labels emitted only when allowlisted via `KH_PROM_LABEL_ALLOWLIST`.
  - `severity` and `category` map from HealthCheck labels `kuberhealthy.io/severity` and `kuberhealthy.io/category`.
  - `run_uuid` maps to the in-flight run UUID and should be allowlisted only when needed.

## Metrics

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `kuberhealthy_cluster_state` | gauge | none | Cluster-wide OK status for all checks. |
| `kuberhealthy_controller_leader` | gauge | none | `1` when this controller instance is the elected leader. |
| `kuberhealthy_scheduler_loop_duration_seconds` | gauge | none | Duration of the most recent scheduling loop. |
| `kuberhealthy_scheduler_due_checks` | gauge | none | Number of checks due to run in the last scheduling loop. |
| `kuberhealthy_reaper_last_sweep_duration_seconds` | gauge | none | Duration of the most recent reaper sweep. |
| `kuberhealthy_reaper_deleted_pods_total` | counter | `reason` | Count of checker pods deleted by the reaper (`timeout`, `completed`, `failed`, `expired`). |
| `kuberhealthy_check` | gauge | `check`, `namespace`, `status`, `error` (optional), extra labels | Health status for each check. |
| `kuberhealthy_check_duration_seconds` | gauge | `check`, `namespace`, extra labels | Duration of the most recent check run. |
| `kuberhealthy_check_consecutive_failures` | gauge | `check`, `namespace`, extra labels | Consecutive failures for the check. |
| `kuberhealthy_check_success_total` | counter | `check`, `namespace`, extra labels | Total successful runs for the check. |
| `kuberhealthy_check_failure_total` | counter | `check`, `namespace`, extra labels | Total failed runs for the check. |
| `kuberhealthy_check_seconds_since_success` | gauge | `check`, `namespace`, extra labels | Seconds since the last OK report (`-1` when unknown). |
| `kuberhealthy_check_run_duration_seconds_bucket` | histogram | `le` | Distribution of last run durations across checks. |
| `kuberhealthy_check_run_duration_seconds_sum` | histogram | none | Sum of last run durations across checks. |
| `kuberhealthy_check_run_duration_seconds_count` | histogram | none | Count of checks included in the histogram. |

## Notes

- Extra labels are drawn from HealthCheck labels and run metadata when allowlisted.
- Avoid allowlisting high-cardinality labels (like `run_uuid`) in large environments.
