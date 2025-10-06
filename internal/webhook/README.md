# internal/webhook

This package exposes helpers for the Kubernetes admission webhook that upgrades
legacy `KuberhealthyCheck` resources to the modern `kuberhealthy.github.io/v2`
`HealthCheck` definition. It accepts admission review payloads, determines
whether conversion is
required, and responds with JSON patches so callers receive an updated resource.
