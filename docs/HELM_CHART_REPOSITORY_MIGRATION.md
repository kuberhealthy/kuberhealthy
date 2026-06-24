# Helm Chart Repository Migration

The Kuberhealthy Helm chart currently lives in this repository at
`deploy/helm/kuberhealthy`. The chart is moving to a dedicated repository so
chart releases can use independent tags and release notes.

## Dedicated Repository

- Repository: `kuberhealthy/kuberhealthy-helm`
- Default branch: `main`
- Chart path: `charts/kuberhealthy`
- Initial chart release: `v1.0.0`
- Initial chart version: `1.0.0`
- Initial chart app version: unchanged from the in-tree chart until a maintainer
  intentionally changes the default image tag behavior

## Published Chart Release

The dedicated chart repository has been created and seeded:

```text
https://github.com/kuberhealthy/kuberhealthy-helm
```

The first independent chart release has also been published:

```text
https://github.com/kuberhealthy/kuberhealthy-helm/releases/tag/v1.0.0
```

That repository contains:

- the extracted chart at `charts/kuberhealthy`
- `Chart.yaml` normalized to `version: 1.0.0`
- chart metadata pointing at `https://github.com/kuberhealthy/kuberhealthy-helm`
- Helm lint/render workflow templates under `workflow-templates/`
- release workflow template for independent `vX.Y.Z` chart tags
- a README with source install, upgrade, release, and migration notes

The workflow files are intentionally stored as templates because the current
GitHub App identity cannot push files under
`.github/workflows` in `kuberhealthy/kuberhealthy-helm`.

## Workflow Installation Follow-Up

An organization owner or token with `workflows` permission should install the
workflow templates in `kuberhealthy/kuberhealthy-helm`:

```bash
git clone https://github.com/kuberhealthy/kuberhealthy-helm.git
cd kuberhealthy-helm
mkdir -p .github/workflows
git mv workflow-templates/helm.yaml .github/workflows/helm.yaml
git mv workflow-templates/release.yaml .github/workflows/release.yaml
rmdir workflow-templates
git commit -m "Install Helm chart workflows"
git push origin main
```

Future chart releases should use semver tags in the chart repository. The
release workflow template validates the tag against `charts/kuberhealthy/Chart.yaml`,
runs `helm lint`, renders the chart, packages the chart, and attaches the
package to the GitHub release for the tag.

## Main Repository Follow-Up

Do not delete `deploy/helm/kuberhealthy` from this repository until the new chart
repository has a published release and users have migration instructions. After
that release exists, this repository can either:

1. keep the in-tree chart temporarily with a deprecation notice, or
2. replace the in-tree chart with a pointer to `kuberhealthy/kuberhealthy-helm`.

The least disruptive path is to keep the in-tree chart for one Kuberhealthy
application release after the external chart repository is published.
