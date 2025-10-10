# Key Structures

Understanding the main structs in the repository clarifies how controller state
is propagated across packages.

## `cmd/kuberhealthy`

- **`Config`** (`config.go`)
  - Holds all runtime settings loaded from environment variables.
  - Fields cover listen addresses, default scheduling intervals, retention
    limits, and TLS options.
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
    recorder, reporting URL, and coordination primitives such as `loopMu`.
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

## `internal/webhook`

- **Conversion helpers** (`convert.go`)
  - `Convert` handles HTTP requests from the Kubernetes API server.
  - `convertReview` and `convertLegacy` transform legacy v1 payloads into v2
    checks, persist them through injected handlers, and return admission
    responses that warn callers while allowing the legacy request to store
    unchanged.
  - `legacyJob` mirrors the legacy `KuberhealthyJob` spec and
    `convertLegacyJobPodSpec` maps its pod template into the modern
    `CheckPodSpec` while forcing `singleRunOnly` on the resulting check.

These structures communicate primarily through the Kubernetes API server and the
shared `Globals` clients, enabling the controller to orchestrate checks while the
HTTP layer provides observability and reporting interfaces.
