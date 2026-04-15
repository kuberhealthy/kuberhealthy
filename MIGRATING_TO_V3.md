# Migrating to Kuberhealthy V3

This guide covers everything you need to know when moving from Kuberhealthy V2 to V3.

⚠️ This is a breaking change.

- V2 is the public release line that teams are running today.
- V3 is the rewritten release line with a new resource model and new install paths.
- Kuberhealthy V2 and V3 are different systems, not small variations of the same release.
- Treat this move as a fresh V3 install plus a migration of your checks and automation.
- Do not expect old resources, old install paths, or old scripts to keep working unchanged.
- Plan time for manifest edits, install changes, and downstream automation updates.
- There is no in-place upgrade path from V2 to V3.
- There is no automatic conversion from the old CRDs to the new CRD.

## High-level steps

- Back up the V2 resources that define your current checks and current state.
- Remove the existing V2 installation before you cut over.
- Install the V3 controller using the new install path you want to standardize on.
- Recreate your checks as `HealthCheck` resources using the new schema.
- Update scripts, dashboards, and automation that still depend on V2 behavior.

## Useful links before you start

- [README.md](README.md) gives the V3 project overview and basic usage.
- [docs/QUICKSTART.MD](docs/QUICKSTART.MD) shows a minimal V3 install and a simple check.
- [docs/CHECK_CREATION.md](docs/CHECK_CREATION.md) explains how to port or build checks.
- [docs/CHECKS_REGISTRY.md](docs/CHECKS_REGISTRY.md) lists ready-to-apply V3 checks.
- [docs/CRD_REFERENCE.MD](docs/CRD_REFERENCE.MD) documents the `HealthCheck` schema.
- [docs/HTTP_API.MD](docs/HTTP_API.MD) documents the V3 endpoints.
- [docs/HELM.MD](docs/HELM.MD) and [docs/KUSTOMIZE.MD](docs/KUSTOMIZE.MD) cover install details.

## What changed at a high level

- V2 used the old CRDs, the old install packaging, and built-in checks bundled into one repo.
- V3 uses a new CRD, new install paths, and separate repositories for built-in checks.
- The core purpose is still the same: run synthetic checks in Kubernetes and export status and metrics.

## What improves in v3

**Simpler resource model**

- One main resource, `HealthCheck`, replaces the older split model.
- No separate `khjob` resource is needed for one-time runs.
- No separate `khstate` resource is needed for persisted check status.

**Better UX**

- A real web UI is available at `/` for quick status checks.
- Machine-readable JSON is served at `/json` for scripts and integrations.
- Manual run, event, and log APIs are available for day-to-day operations.

**Cleaner install story**

- A Kustomize base is available for manifest-first installs.
- An in-repo Helm chart replaces the old external Helm packaging flow.
- A clear Argo CD entrypoint is available for GitOps users.

**Better check packaging**

- Built-in checks now live in their own repositories instead of being bundled here.
- Checks are easier to version, release, and consume independently from the controller.

## What stays the same

- Kuberhealthy still runs synthetic checks inside Kubernetes.
- Check pods still report their result back using `KH_REPORTING_URL`.
- Prometheus metrics are still exposed at `/metrics`.
- You still define checks as Kubernetes manifests applied to the cluster.

## Quick facts

- The main V2 check resource was `KuberhealthyCheck`.
- The main V3 check resource is `HealthCheck`.
- V2 used the API group `comcast.github.io/v1`.
- V3 uses the API group `kuberhealthy.github.io/v2`.
- V2 used `KuberhealthyJob` for one-shot execution.
- V3 uses `HealthCheck` with `singleRunOnly: true` for one-shot execution.
- V2 stored status in `KuberhealthyState`.
- V3 stores status directly in `HealthCheck.status`.

## What you need to do

- Back up your old `khcheck`, `khjob`, and `khstate` resources before touching the install.
- Remove the old installation once you are ready to cut over.
- Install V3 using Kustomize, the in-repo Helm chart, or the new Argo CD manifest.
- Recreate checks as `HealthCheck` resources instead of reusing the old CRDs.
- Update anything that reads old status resources or old HTTP endpoints.

## Resource changes

### Old resources are gone

- `KuberhealthyCheck` does not exist in V3.
- `KuberhealthyJob` does not exist in V3.
- `KuberhealthyState` does not exist in V3.

### New resource

- `HealthCheck` is the resource you use for scheduled and one-shot checks in V3.

### Command changes

- Old: `kubectl get khcheck`
- New: `kubectl get healthcheck`

- Old: `kubectl get khstate`
- New: read the check result from `HealthCheck.status`

## Manifest changes

### Old

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
spec:
  runInterval: 30s
  timeout: 2m
  podSpec:
    containers:
      - name: main
        image: my-check:latest
    restartPolicy: Never
```

### New

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: HealthCheck
spec:
  runInterval: 30s
  timeout: 2m
  podSpec:
    spec:
      containers:
        - name: main
          image: my-check:latest
      restartPolicy: Never
```

### Required edits

- Change the `apiVersion` to the V3 API group and version.
- Change the `kind` from `KuberhealthyCheck` to `HealthCheck`.
- Move pod fields under `spec.podSpec.spec` instead of placing them directly under `podSpec`.
- Move pod labels and annotations under `spec.podSpec.metadata`.
- Replace any `KuberhealthyJob` usage with `singleRunOnly: true`.

### Useful manifest examples

- [docs/healthchecks/DAEMONSET_CHECK.yaml](docs/healthchecks/DAEMONSET_CHECK.yaml) is a full V3 daemonset check example.
- [docs/healthchecks/DEPLOYMENT_CHECK.yaml](docs/healthchecks/DEPLOYMENT_CHECK.yaml) is a full V3 deployment check example.

## Check install changes

### Old behavior

- This repo shipped the controller and many built-in checks together.
- The old Helm chart could install built-in checks for you as part of the controller install.
- Default old chart checks included:
  - `daemonset`
  - `deployment`
  - `dns-internal`

### New behavior

- This repo now ships the controller only.
- Built-in checks now live in separate repos under the `kuberhealthy` GitHub organization.
- Recreate the checks you still want instead of expecting them to appear automatically.
- The V3 Helm chart does not install the old default checks.

Useful built-in check repos:

- [kuberhealthy/daemonset-check](https://github.com/kuberhealthy/daemonset-check)
- [kuberhealthy/deployment-check](https://github.com/kuberhealthy/deployment-check)
- [kuberhealthy/dns-resolution-check](https://github.com/kuberhealthy/dns-resolution-check)

## Install changes

### Old install paths

- Flat file install:

```sh
kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/master/deploy/kuberhealthy.yaml
```

- Old Helm repo:

```sh
helm repo add kuberhealthy https://kuberhealthy.github.io/kuberhealthy/helm-repos
helm install -n kuberhealthy kuberhealthy kuberhealthy/kuberhealthy
```

### New install paths

- Kustomize:

```sh
kubectl apply -k github.com/kuberhealthy/kuberhealthy/deploy/kustomize/base?ref=main
```

- Helm (requires the repo to be cloned locally):

```sh
helm install kuberhealthy deploy/helm/kuberhealthy -n kuberhealthy --create-namespace
```

- Argo CD (requires the repo to be cloned locally):

```sh
kubectl apply -f deploy/argocd/kuberhealthy.yaml
```

### Important install difference

- The Kustomize base includes an `example-check` resource.
- The Helm chart does not include an example check by default.
- If you use the Kustomize base, remove `example-check` if you do not want a sample check running in the cluster.

## Config changes

### Old

- Controller config came from a ConfigMap-backed YAML file.
- Many users changed config with:

```sh
kubectl edit -n kuberhealthy configmap kuberhealthy
```

### New

- Controller config is environment variables only.
- Do not use the old ConfigMap editing flow during V3 operations.
- Update the Deployment environment variables instead.
- Old InfluxDB forwarding support is gone.

### Important env var rename

- Old: `TARGET_NAMESPACE`
- New: `KH_TARGET_NAMESPACE`

- Old: `KH_EXTERNAL_REPORTING_URL`
- New: `KH_CHECK_REPORT_URL`

### Important

- `KH_CHECK_REPORT_URL` should contain the base URL only.
- Do not include `/check` in that value.

## Endpoint changes

### Status

- Old JSON endpoint: `/`
- New JSON endpoint: `/json`
- New UI endpoint: `/`

If you have scripts that call `/` and expect JSON output, update them to call `/json`.

### Reporting

- Old report endpoint: `/externalCheckStatus`
- New report endpoint: `/check`

Check pods still receive:

- `KH_REPORTING_URL`
- `KH_CHECK_RUN_DEADLINE`
- `KH_RUN_UUID`
- `KH_POD_NAMESPACE`

## Status changes

### Old

- Status lived in `khstate`

### New

- Status now lives directly in `HealthCheck.status`.

### Also changed

- The old JSON detail shape is not the same as the new JSON detail shape.
- Review any automation that parses per-check fields or expects old field names.

## Check code changes

### Go import path

Old:

```go
github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient
```

New:

```go
github.com/kuberhealthy/kuberhealthy/v3/pkg/checkclient
```

### Other languages

- The old repo had local client examples for some languages.
- The new docs point users to separate language-specific repositories.

## Important defaults to review

⚠️ Set these explicitly if you depended on old behavior.

- Old default check timeout: `5m`
- New default check timeout: `30s`

- Old Helm service default: `ClusterIP`
- New Helm service default: `LoadBalancer`

- New V3 default controller replicas: `2`
- New V3 default controller anti-affinity: hard node spread

## Simple migration flow

### 1. Back up old resources

```sh
kubectl get khchecks,khjobs,khstates -A -o yaml > kuberhealthy-v2-backup.yaml
```

If you use Helm:

```sh
helm get values -n kuberhealthy kuberhealthy -o yaml > kuberhealthy-v2-values.yaml
```

### 2. Rewrite your manifests

- Convert each `KuberhealthyCheck` into a `HealthCheck`.
- Replace each `KuberhealthyJob` with a `HealthCheck` that uses `singleRunOnly: true`.
- Move pod fields under `spec.podSpec.spec`.
- Set `timeout` explicitly if the old defaults mattered to you.

### 3. Remove the old install

- Remove the old Helm release, flat-file install, or GitOps source.

If you want to clean up the old CRDs after the migration is complete:

```sh
kubectl delete crd khchecks.comcast.github.io
kubectl delete crd khjobs.comcast.github.io
kubectl delete crd khstates.comcast.github.io
```

- This cleanup is optional during the migration itself.
- V2 and V3 use different CRD names, so you can remove the old CRDs after V3 is live and you no longer need the V2 objects.

### 4. Install v3

- Use Kustomize, the in-repo Helm chart, or the new Argo CD manifest.

### 5. Apply your new `HealthCheck` resources

- Recreate the built-in checks you still want to run.
- Reapply your custom checks as `HealthCheck` resources.

### 6. Update automation

- Change any `/` JSON readers to `/json`.
- Move status readers off `khstate`.
- Update old Go client imports to the V3 path.
- Update old install commands and old Helm repo usage.
- If you use Prometheus Operator, apply `deploy/serviceMonitor.yaml`.

## Common mistakes

- Expecting old CRDs to work in v3
- Copying the old `podSpec` shape into v3 (pod fields must go under `spec.podSpec.spec`)
- Forgetting that `/` is now the UI — JSON is at `/json`
- Setting `KH_CHECK_REPORT_URL` to a value ending in `/check`
- Expecting the v3 chart to install old default checks
- Relying on the old 5 minute timeout default (v3 default is `30s`)
- Forgetting that v3 defaults to 2 controller replicas
- Forgetting that the Kustomize base creates an `example-check` resource

## Final checklist

- Back up the old resources before touching the install.
- Use a fresh V3 install instead of trying to upgrade V2 in place.
- Convert manifests to `HealthCheck`.
- Recreate any built-in checks you still need.
- Move JSON readers to `/json`.
- Move status readers off `khstate`.
- Review timeout and service defaults before rollout.
- Review replica count and service exposure before rollout.

## More V3 docs

- [docs/RUN_ONCE_CHECKS.MD](docs/RUN_ONCE_CHECKS.MD) covers one-shot checks in V3.
- [docs/FLAGS.md](docs/FLAGS.md) lists controller environment variables.
- [docs/prometheus/SERVICE_MONITOR.MD](docs/prometheus/SERVICE_MONITOR.MD) covers Prometheus Operator setups.
