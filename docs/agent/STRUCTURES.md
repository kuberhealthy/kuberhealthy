# Key Structures

Understanding the main structs in the repository clarifies how controller state
is propagated across packages.

## `cmd/kuberhealthy`

- **`Config`** (`config.go`)
  - Holds all runtime settings loaded from environment variables.
  - Fields cover listen addresses, default scheduling intervals, retention
    limits, and TLS options.
  - Leader election fields (enablement, lease name/namespace, and timing
    durations) control Lease-based failover behavior.
  - `ReportingURL()` derives the absolute `/check` endpoint advertised to pods.
- **`Globals`** (`main.go`)
  - Struct literal containing the Kubernetes REST config, Kubernetes clientset,
    controller-runtime client, and the active `*kuberhealthy.Kuberhealthy`
    instance.
  - Shared across HTTP handlers so they can perform CRUD operations.

## `internal/kuberhealthy`

- **`Kuberhealthy`** (`kuberhealthy.go`)
  - Encapsulates the scheduling logic for `HealthCheck` resources.
  - Stores the process context, cancellation function, Kubernetes client, event
    recorder, reporting URL, leader state, and coordination primitives such as
    `loopMu`.
  - Captures controller metrics snapshots (scheduler loop timing, due checks,
    reaper sweep timing, and deletion counts) for Prometheus export.
  - Exposes methods for starting and stopping the scheduler, beginning specific
    runs, tracking timeouts, and validating report UUIDs.
- **`checkRun` / `activeCheck` helpers**
  - Maintain in-memory bookkeeping for currently executing checks, ensuring
    duplicate runs do not overlap and that timeout cleanup can resume after a
    restart.

## `pkg/api`

- **`HealthCheck`**
  - Custom resource definition representing a synthetic check.
  - `Spec` embeds a `CheckPodSpec` that includes Kubernetes pod metadata and the
    exact `PodSpec` to run, while `Status` captures runtime results.
  - `Status` now includes success/failure counters and timestamps for the last
    OK or failed run, enabling SLO-focused metrics.
- **`CheckPodSpec` and `CheckPodMetadata`**
  - Define the pod template executed for each run, plus optional labels and
    annotations applied by the controller.
- **Helper Functions**
  - Functions such as `GetCheck`, `UpdateCheck`, and `SetCurrentUUID` wrap common
    CRUD tasks and guarantee consistent error handling across packages.

## `internal/metrics`

- **`Exporter`** (`exporter.go`)
  - Periodically reads the stored check status and emits Prometheus gauges and
    counters describing run health.
  - Depends on the shared `Globals.kh` instance to query state.

These structures communicate primarily through the Kubernetes API server and the
shared `Globals` clients, enabling the controller to orchestrate checks while the
HTTP layer provides observability and reporting interfaces.
