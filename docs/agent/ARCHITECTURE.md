# Architecture

Kuberhealthy runs as a controller process inside a Kubernetes cluster.
Multiple replicas may serve the status and admin HTTP surfaces, while a single
leader (selected via a Kubernetes Lease) runs scheduling, watches, and reaping.
The binary defined in `cmd/kuberhealthy` composes several internal packages to
watch custom resources, orchestrate synthetic check pods, and publish
observability interfaces.

```
+-------------------+        +-----------------------+        +-----------------+
| Kubernetes API    |<------>| internal/kuberhealthy |<------>| internal/envs   |
|  - healthchecks   |  CRUD  |  - scheduling loop    |  env   |  - runtime cfg  |
|  - pods/events    |        |  - timeout tracking   |        +-----------------+
|  - leases         |        |  - status updates     |                  |
+-------------------+        +-----------------------+                  v
         ^                             |                      +---------------------+
         | reports via HTTP            | spawns               | internal/metrics    |
         |                             v                      |  - Prometheus exp.  |
+-------------------+        +-----------------------+        +---------------------+
| Check Pods        | -----> | cmd/kuberhealthy      | -----> | cmd/kuberhealthy    |
|  - custom logic   |  POST  |  - web handlers       |  serve |  - status + report  |
+-------------------+  /check |  - leader election   |  HTTP  +---------------------+
                               \-------------------------------/
```

Key components:

- **`cmd/kuberhealthy`** wires together configuration, leader election, the
  core controller, and the HTTP server. All replicas serve `/json`, `/metrics`,
  `/check`, the OpenAPI specification, and helper endpoints. Only the elected
  leader runs the scheduler, watch, and reaper loops.
- **`internal/kuberhealthy`** implements the scheduling engine. It watches for
  `HealthCheck` resources, starts checker pods, tracks run lifecycles, and
  writes results back to the Kubernetes API.
- **`internal/envs`** collects configuration from environment variables so the
  controller and web server share consistent defaults.
- **`internal/metrics`** publishes Prometheus metrics and reuses the stored
  status for gauge generation.
- **`pkg/api`** defines the custom resource types for the Kubernetes API server
  and includes helpers for CRUD operations.

All long-running routines derive from the root context managed in `main`. When
shutdown begins, the context cancels, goroutines unwind, and the controller
signals the main thread through a completion channel so the process can exit
cleanly after any remaining work drains.
