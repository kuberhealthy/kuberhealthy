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

1. Check pods submit their status to the `/report` endpoint on the HTTP server.
2. `checkReportHandler` validates the payload, verifies the run UUID, and writes
   the success flag and any error strings into the `status` block of the
   `HealthCheck` resource.
3. `internal/kuberhealthy` records the run metadata (duration, timestamps, and
   failure counts) so subsequent scheduling decisions can skip recent runs and
   inform the Prometheus exporter.

## Metrics and Status Surfaces

1. The `/status` endpoint renders a JSON document summarizing each known check
   using the data persisted in the `status` block.
2. `internal/metrics/exporter.go` constructs Prometheus metrics from the stored
   results, exposing `kuberhealthy_check_status` and related series via the
   `/metrics` endpoint.
3. Optional helper endpoints allow triggering runs on-demand, streaming pod
   logs, and downloading the OpenAPI schema, all of which reuse the shared
   `Globals` clients.

## Legacy Conversion

When the Kubernetes API server sends an admission review to the legacy
conversion webhook, `internal/webhook` inspects the payload. Legacy
`comcast.github.io/v1` resources are converted into the modern `v2` schema. The
webhook now upserts a `kuberhealthy.github.io/v2/HealthCheck` resource with the
translated specification and schedules a background cleanup loop that removes
the original `khchecks.comcast.github.io` object once it has been persisted. The
webhook now allows the legacy admission to proceed unchanged while emitting a
warning, relying on the background cleanup job to delete the v1 object after
the modern resource exists. This keeps legacy manifests functional without
requiring the AdmissionReview response to rewrite the object into a different
API group.

The mutating webhook relies on TLS to serve the Kubernetes API server. Each
cluster must generate its own serving certificate and CA bundle so the API
server trusts the hook. The helper script at
`deploy/base/scripts/generateWebhookcert.sh` (or the job definition in
`deploy/base/scripts/webhookCertJob.yaml`) creates a namespace-scoped secret and
updates the webhook's `caBundle`. Operations should re-run the script whenever
the HTTPS secret needs rotation so the API server continues to accept the hook.
With `failurePolicy: Fail`, the API server now rejects legacy objects if the
conversion webhook is unavailable, ensuring that unconverted payloads never hit
storage. The service exposes HTTPS on port `443` exclusively for webhook traffic
while port `8080` continues to serve the public HTTP endpoints.
