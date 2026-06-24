# Helm Chart Repository Migration

The Kuberhealthy Helm chart currently lives in this repository at
`deploy/helm/kuberhealthy`. The chart should move to a dedicated repository so
chart releases can use independent tags and release notes.

## Proposed Repository

- Repository: `kuberhealthy/helm-chart`
- Default branch: `main`
- Chart path: `charts/kuberhealthy`
- Initial chart version: `1.0.0`, matching this repository's normalized chart
  metadata
- Initial chart app version: unchanged from the current chart until a maintainer
  intentionally changes the default image tag behavior

## Prepared Seed Branch

A seed branch has been pushed to this repository:

```text
https://github.com/kuberhealthy/kuberhealthy/tree/helm-chart-repo-seed
```

That branch contains:

- the extracted chart at `charts/kuberhealthy`
- `Chart.yaml` normalized to `version: 1.0.0`
- chart metadata pointing at `https://github.com/kuberhealthy/helm-chart`
- Helm lint/render workflow templates under `workflow-templates/`
- release workflow template for independent `vX.Y.Z` chart tags
- a README with source install, upgrade, release, and migration notes

The workflow files are intentionally stored as templates on the seed branch
because the current GitHub App identity cannot push files under
`.github/workflows` in `kuberhealthy/kuberhealthy`.

## Maintainer Bootstrap Steps

An organization owner must create the new repository first. The current
Kuberhealthy Coder and Personal PAT integrations both returned:

```text
You need admin access to the organization before adding a repository to it.
```

After `kuberhealthy/helm-chart` exists, bootstrap it from the prepared seed
branch:

```sh
git clone https://github.com/kuberhealthy/kuberhealthy.git helm-chart
cd helm-chart
git checkout helm-chart-repo-seed
git remote set-url origin https://github.com/kuberhealthy/helm-chart.git
git push -u origin HEAD:main
```

Then install the workflow templates in the new repository:

```sh
mkdir -p .github/workflows
git mv workflow-templates/helm.yaml .github/workflows/helm.yaml
git mv workflow-templates/release.yaml .github/workflows/release.yaml
rmdir workflow-templates
git commit -m "Install Helm chart workflows"
git push origin main
```

## First Chart Release

Once the new repository is populated, Release Manager can publish the first
independent chart release:

```sh
git tag v1.0.0
git push origin v1.0.0
```

The release workflow template validates the tag against
`charts/kuberhealthy/Chart.yaml`, runs `helm lint`, renders the chart, packages
the chart, and attaches the package to the GitHub release for the tag.

## Main Repository Follow-Up

Do not delete `deploy/helm/kuberhealthy` from this repository until the new chart
repository has a published release and users have migration instructions. After
that release exists, this repository can either:

1. keep the in-tree chart temporarily with a deprecation notice, or
2. replace the in-tree chart with a pointer to `kuberhealthy/helm-chart`.

The least disruptive path is to keep the in-tree chart for one Kuberhealthy
application release after the external chart repository is published.
