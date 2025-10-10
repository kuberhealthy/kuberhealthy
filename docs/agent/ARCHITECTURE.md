# Architecture

Kuberhealthy runs as a single controller process inside a Kubernetes cluster.
The binary defined in `cmd/kuberhealthy` composes several internal packages to
watch custom resources, orchestrate synthetic check pods, and publish
observability interfaces.

```
+-------------------+        +-----------------------+        +-----------------+
| Kubernetes API    |<------>| internal/kuberhealthy |<------>| internal/envs   |
|  - healthchecks   |  CRUD  |  - scheduling loop    |  env   |  - runtime cfg  |
|  - pods/events    |        |  - timeout tracking   |        +-----------------+
+-------------------+        |  - status updates     |                  |
         ^                   +-----------------------+                  v
         |                             |                      +---------------------+
         | reports via HTTP            | spawns               | internal/metrics    |
         |                             v                      |  - Prometheus exp.  |
+-------------------+        +-----------------------+        +---------------------+
| Check Pods        | -----> | cmd/kuberhealthy      | -----> | cmd/kuberhealthy    |
|  - custom logic   |  POST  |  - web handlers       |  serve |  - status + report  |
+-------------------+  /report|  - report ingestion  |  HTTP  +---------------------+
                               \-------------------------------/
                                             |
                                  +---------------------+
                                  | internal/webhook    |
                                  |  - legacy CRD conv. |
                                  |  - khjob->v2 map    |
                                  +---------------------+
```

Key components:

- **`cmd/kuberhealthy`** wires together configuration, the core controller, and
  the HTTP server. It exposes `/status`, `/metrics`, `/report`, the OpenAPI
  specification, and helper endpoints for running checks and inspecting pod
  output.
- **`internal/kuberhealthy`** implements the scheduling engine. It watches for
  `HealthCheck` resources, starts checker pods, tracks run lifecycles, and
  writes results back to the Kubernetes API.
- **`internal/envs`** collects configuration from environment variables so the
  controller and web server share consistent defaults.
- **`internal/metrics`** publishes Prometheus metrics and reuses the stored
  status for gauge generation.
- **`internal/webhook`** upgrades legacy `comcast.github.io/v1` checks and
  `kuberhealthy.comcast.io/v1` jobs to the v2 CRD schema via a Kubernetes
  admission webhook, forcing converted jobs to run as single-execution health
  checks.
- **`pkg/api`** defines the custom resource types for the Kubernetes API server
  and includes helpers for CRUD operations.

All long-running routines derive from the root context managed in `main`. When
shutdown begins, the context cancels, goroutines unwind, and the controller
signals the main thread through a completion channel so the process can exit
cleanly after any remaining work drains.
