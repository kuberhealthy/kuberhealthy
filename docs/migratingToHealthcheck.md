# Migrating Kuberhealthy v2 to v3

Kuberhealthy v3 is a full rewrite with a new controller, API surface, and CRD.
The legacy v2 CRDs (`KuberhealthyCheck`, `KuberhealthyJob`, and
`KuberhealthyState`) are not valid in v3, and there is no in-place migration or
conversion webhook. We want this to be predictable and safe, so the upgrade
path is a clean reinstall and a recreate of your checks using the v3 schema.

## What to expect

- v2 CRDs cannot be read or reconciled by v3, including `KuberhealthyState`.
- Existing KHChecks need to be rewritten as v3 `HealthCheck` resources.
- Your check report URL must use the v3 endpoint `/check`, and
  `KH_CHECK_REPORT_URL` should be the base URL only.

## Recommended upgrade steps

- Remove the v2 installation and its legacy CRDs after all legacy resources are gone.
- Deploy v3 into the desired namespace (Kustomize, Helm, or ArgoCD).
- Recreate checks as v3 `HealthCheck` resources using the new schema.
