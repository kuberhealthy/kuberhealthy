# kuberhealthy

This package contains the core controller that orchestrates Kuberhealthy checks. It schedules check executions, watches `HealthCheck` resources, reaps stale pods, finalizes completed runs, and records Kubernetes events.

The package is responsible for coordinating check lifecycles and overall controller operation. Lower-level helpers and data definitions live in other internal and pkg packages.
