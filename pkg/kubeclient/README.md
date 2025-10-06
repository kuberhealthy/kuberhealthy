# kubeclient

The kubeclient package creates controller-runtime clients that understand Kuberhealthy's custom resources. It offers `NewClient` to build a `client.Client` with Kuberhealthy CRDs registered and a higher level `KHClient` wrapper with CRUD helpers for `HealthCheck` objects.

This package is responsible only for constructing and wrapping Kubernetes clients. Scheduling, execution, and business logic for checks are handled elsewhere.
