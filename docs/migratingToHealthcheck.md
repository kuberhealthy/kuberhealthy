# Migrating Kuberhealthy v2 to v3

Kuberhealthy v3 is a breaking release and does not include migration tooling or conversion webhooks. Plan for a full redeploy.

## What to do

- Remove the v2 installation and its legacy CRDs after all legacy resources are gone.
- Deploy v3 into the desired namespace (Kustomize, Helm, or ArgoCD).
- Recreate checks as v3 `HealthCheck` resources using the new schema.
- Update your check report URL to use the v3 endpoint `/check`; `KH_CHECK_REPORT_URL` must be the base URL only.
