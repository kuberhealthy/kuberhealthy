# Troubleshooting

## Metrics missing

- Confirm the service is reachable: `curl -fsS localhost:8080/metrics`.
- Ensure the pod is running: `kubectl -n kuberhealthy get pods`.

## Checks stuck or failing

- Inspect the `HealthCheck` status: `kubectl -n kuberhealthy describe healthcheck <name>`.
- Review checker pod logs for the last run.
- Confirm the check reports to `KH_REPORTING_URL` before `KH_CHECK_RUN_DEADLINE`.

## JSON status page not reachable

- Port-forward the service: `kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:8080`.
- If using Helm, the service name is `kuberhealthy`.

If you are blocked, please open an issue with the output of the commands above.
