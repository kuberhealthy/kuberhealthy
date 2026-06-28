# Helm Chart Repository Migration

Kuberhealthy uses a dedicated Helm chart repository for independent chart
releases. This repository keeps the in-tree chart at `deploy/helm/kuberhealthy`
as a source install path and compatibility reference.

## Dedicated Repository

- Repository: `kuberhealthy/kuberhealthy-helm`
- Default branch: `main`
- Chart path: `charts/kuberhealthy`
- Release: `v1.0.1`
- Chart version: `1.0.1`
- App version: `v3.0.4`

## Published Release

The independent chart release is published at:

```text
https://github.com/kuberhealthy/kuberhealthy-helm/releases/tag/v1.0.1
```

Release `v1.0.1` uses chart version `1.0.1` and app version `v3.0.4`.
The Helm repository index is published from the chart repository default branch:

```sh
helm repo add kuberhealthy https://raw.githubusercontent.com/kuberhealthy/kuberhealthy-helm/main
helm repo update
```

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

This repository includes the chart at:

```text
deploy/helm/kuberhealthy
```

Use this path for local source-tree installs, examples, and development against
this repository. Use `kuberhealthy/kuberhealthy-helm` for independent Helm chart
releases and chart-specific release notes.

## Migration For Existing Installs

Use the dedicated chart repository for existing installs that were created from
`deploy/helm/kuberhealthy`. The chart name stays `kuberhealthy`, so the release
can be upgraded in place.

First, add the chart repository and save the values from the installed release:

```sh
helm repo add kuberhealthy https://raw.githubusercontent.com/kuberhealthy/kuberhealthy-helm/main
helm repo update
helm get values kuberhealthy \
  -n kuberhealthy \
  -o yaml > kuberhealthy-values.yaml
```

Then capture the image that is running before the chart-source migration. This
keeps the rendered Deployment image stable even when the old install relied on
the in-tree chart default `appVersion: main` and the dedicated chart uses
`appVersion: v3.0.4`.

```sh
KUBERHEALTHY_IMAGE="$(kubectl -n kuberhealthy get deployment kuberhealthy \
  -o jsonpath='{.spec.template.spec.containers[?(@.name=="kuberhealthy")].image}')"
```

Run a dry-run render with the dedicated chart and inspect the output before
applying it:

```sh
helm upgrade kuberhealthy kuberhealthy/kuberhealthy \
  -n kuberhealthy \
  --version 1.0.1 \
  -f kuberhealthy-values.yaml \
  --set-string imageURL="${KUBERHEALTHY_IMAGE}" \
  --dry-run
```

Apply the in-place migration with the same inputs:

```sh
helm upgrade kuberhealthy kuberhealthy/kuberhealthy \
  -n kuberhealthy \
  --version 1.0.1 \
  -f kuberhealthy-values.yaml \
  --set-string imageURL="${KUBERHEALTHY_IMAGE}"
```

Verify that the release and Deployment are healthy:

```sh
helm status kuberhealthy -n kuberhealthy
helm get manifest kuberhealthy -n kuberhealthy | kubectl apply --dry-run=server -f -
kubectl -n kuberhealthy rollout status deployment/kuberhealthy
kubectl -n kuberhealthy get deployment kuberhealthy \
  -o jsonpath='{.spec.template.spec.containers[?(@.name=="kuberhealthy")].image}{"\n"}'
```

If the release uses a custom namespace, release name, or `nameOverride`, replace
`kuberhealthy` in the commands with the installed release, namespace, or
Deployment name. If the release already sets `imageURL` or `image.tag`
intentionally, keep that value in `kuberhealthy-values.yaml` and omit the
`--set-string imageURL=...` override when it would conflict with the desired
image policy.

## Compatibility Path

The in-tree chart at `deploy/helm/kuberhealthy` remains available as a source
install path, examples target, and compatibility reference for this repository.
It is not a wrapper around the dedicated chart repository. Chart release tags,
packaged chart artifacts, and chart-specific release notes are owned by
`kuberhealthy/kuberhealthy-helm`.
