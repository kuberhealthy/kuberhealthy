# Logic Overview

Kuberhealthy's runtime revolves around four primary flows that start in
`cmd/kuberhealthy` and fan out through the supporting packages.

## Startup and Configuration

1. `main` loads configuration via `setUp`, which reads environment variables
   into the `GlobalConfig` struct defined in `cmd/kuberhealthy/config.go`.
2. Kubernetes REST clients and the controller-runtime client are created once
   so the process can watch `HealthCheck` resources and interact with
   pods, events, and other cluster state.
3. `kuberhealthy.New` creates the controller instance, wiring in the context and
   optional shutdown notifier channel used later during graceful termination.
4. `Globals.kh.Start` launches goroutines for scheduling, timeout recovery, and
   CRD watches. Any startup error is logged so the process continues to expose
   diagnostics.
5. `StartWebServer` builds the HTTP multiplexer and starts HTTP/TLS listeners
   for status pages, reporting, metrics, and helper endpoints.

## Scheduling and Running Checks

1. `startScheduleLoop` in `internal/kuberhealthy` evaluates known checks and
   determines whether any need to run based on the stored status and the
   configured intervals.
2. When a run is due, the controller creates a `Pod` derived from the check's
   embedded `PodSpec`, annotating it with identifying labels and run metadata.
3. The controller monitors the pod lifecycle. It enforces timeouts with a small
   grace period and records Kubernetes events that describe failures.
4. Once a pod exits successfully, the controller waits for a report from the pod
   before marking the run complete.

## Reporting Results

1. Check pods submit their status to the `/check` endpoint on the HTTP server.
2. `checkReportHandler` validates the payload, verifies the run UUID, and writes
   the success flag and any error strings into the `status` block of the
   `HealthCheck` resource.
3. `internal/kuberhealthy` records the run metadata (duration, timestamps, and
   failure counts) so subsequent scheduling decisions can skip recent runs and
   inform the Prometheus exporter.

## Metrics and Status Surfaces

1. The `/json` endpoint renders a JSON document summarizing each known check
   using the data persisted in the `status` block.
2. `internal/metrics/exporter.go` constructs Prometheus metrics from the stored
   results, exposing `kuberhealthy_check_status` and related series via the
   `/metrics` endpoint.
3. Optional helper endpoints allow triggering runs on-demand, streaming pod
   logs, and downloading the OpenAPI schema, all of which reuse the shared
   `Globals` clients.

## Legacy Conversion

Kuberhealthy v3 does not ship conversion webhooks or backward compatibility
paths. Legacy resources must be removed and recreated as `HealthCheck` objects.
