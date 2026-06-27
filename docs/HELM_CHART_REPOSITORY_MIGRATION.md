# Helm Chart Repository Migration

Kuberhealthy now has a dedicated Helm chart repository for independent chart
releases. This repository keeps the in-tree chart at `deploy/helm/kuberhealthy`
as a source install path and compatibility reference.

## Dedicated Repository

- Repository: `kuberhealthy/kuberhealthy-helm`
- Default branch: `main`
- Chart path: `charts/kuberhealthy`
- Current release: `v1.0.1`
- Chart version: `1.0.1`
- App version: `v3.0.4`
- Previous release: `v1.0.0`

## Published Release

The current independent chart release is published at:

```text
https://github.com/kuberhealthy/kuberhealthy-helm/releases/tag/v1.0.1
```

Release `v1.0.1` supersedes the initial `v1.0.0` chart release and includes the
final validation fixes with `appVersion: v3.0.4`.

## Repository Contents

The dedicated chart repository contains:

- the chart at `charts/kuberhealthy`
- `Chart.yaml` with `version: 1.0.1` and `appVersion: v3.0.4`
- chart metadata pointing at `https://github.com/kuberhealthy/kuberhealthy-helm`
- Helm lint/render workflow templates under `workflow-templates/`
- a release workflow template for independent `vX.Y.Z` chart tags
- README and release documentation for source install, upgrade, release, and
  migration workflows

## Source Tree Chart

This repository still includes the chart at:

```text
deploy/helm/kuberhealthy
```

Use this path for local source-tree installs, examples, and development against
this repository. Use `kuberhealthy/kuberhealthy-helm` for independent Helm chart
releases and chart-specific release notes.
