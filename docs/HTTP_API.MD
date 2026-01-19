# HTTP API and Endpoints

This document covers the HTTP surfaces exposed by the Kuberhealthy controller
service and the reporting endpoint used by check pods.

## Status UI and JSON summary

- `GET /` serves the HTML status UI.
- `GET /json` returns the JSON summary of all HealthChecks. Use `?namespace=<ns1,ns2>` to filter.

## Metrics and health probes

- `GET /metrics` returns Prometheus metrics.
- `GET /healthz` returns controller liveness status.

## Check reporting

Checker pods report to the `/check` endpoint. The controller injects these
environment variables into every check pod:

- `KH_REPORTING_URL`: Full URL for reporting check results (must end with `/check`).
- `KH_RUN_UUID`: Run identifier to send as the `kh-run-uuid` header.
- `KH_CHECK_RUN_DEADLINE`: Unix timestamp deadline for reporting.
- `KH_POD_NAMESPACE`: Namespace the checker pod is running in.

The request must be a JSON payload with `ok` and `errors` fields (case-insensitive), for example:

```json
{
  "ok": true,
  "errors": []
}
```
Set `Content-Type: application/json`.

## Manual run and log access

- `POST /api/run?namespace=<namespace>&healthcheck=<healthcheck>` triggers an immediate
  run of a HealthCheck.
- `GET /api/events?namespace=<namespace>&healthcheck=<healthcheck>` returns Kubernetes
  events for a HealthCheck.
- `GET /api/logs?namespace=<namespace>&healthcheck=<healthcheck>&pod=<pod>&format=text`
  fetches pod logs for a check run.
- `GET /api/logs/stream?namespace=<namespace>&healthcheck=<healthcheck>&pod=<pod>`
  streams logs.

## OpenAPI schema

- `GET /openapi.yaml` serves the OpenAPI schema in YAML format.
- `GET /openapi.json` serves the OpenAPI schema in JSON format.
