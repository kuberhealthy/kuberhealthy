# HealthCheck (legacy package name `khcheck`)

This package defines the internal representation of a Kuberhealthy HealthCheck. The Go package still uses the legacy `khcheck` name for import stability, but the exported `KHCheck` struct models the scheduling parameters, runtime metadata, and cancellation controls for an individual check pod.

This package is limited to the data structure and related synchronization primitives. Logic that schedules or executes checks resides in the `kuberhealthy` controller package.
