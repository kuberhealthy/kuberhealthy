# Release Notes

## v3 highlights

- Leader election is available for multi-replica controller deployments. Only the leader runs checks and reaps pods, while all replicas serve the UI and APIs.
- Leader election is configured with `KH_LEADER_ELECTION_*` environment variables and requires Lease RBAC (`coordination.k8s.io` `leases`).
- Built-in checks (daemonset, deployment, and others) now live in their own repositories under the `kuberhealthy` GitHub organization.
- Prometheus metrics include controller health, run counters, and a versioned catalog in `docs/METRICS_CATALOG.MD`.

## Upgrade notes

- v3 uses a new `HealthCheck` CRD. Plan a clean reinstall and recreate checks using the v3 schema.
