# Migrating Kuberhealthy v2 (master branch) to v3 (main branch)

This document will guide you through upgrading to Kuberhealthy V3 from an existing V2 installation. Kuberhealthy V3 comes with a new CRD (healthcheck) that replaces both `kuberhealthycheck` and `kuberhealthyjob`. Additionally, V3 provides a new built-in web interface that can be used to see check status and runs in real time.

Because of the significance of these changes, it is expected for you to install V3 alongside V2, migrate your checks, verify all functionality, and then remove V2. Your existing check containers will still work without rewrite, but you'll need to deploy them via the new `healthcheck` custom resource.

## 1) Install v3 alongside v2

- Use a new namespace and a name suffix so cluster-scoped RBAC objects do not collide.
- Deploy v3 from `deploy/kustomize/base` (v3 uses Kustomize, not the v2 flat YAML). 
#TODO - give example kustomize command to install directly off github to a cluster
- Update `KH_CHECK_REPORT_URL` to point at the new Service name/namespace.

Example Kustomize overlay:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: kuberhealthy-v3
nameSuffix: -v3
resources:
  - ../base
patchesStrategicMerge:
  - deploymentPatch.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kuberhealthy
spec:
  template:
    spec:
      containers:
        - name: kuberhealthy
          env:
            - name: KH_CHECK_REPORT_URL
              value: "http://kuberhealthy-v3.kuberhealthy-v3.svc.cluster.local:8080"
```

Apply it:

```sh
kubectl apply -k <your-overlay-dir>
```

## 2) Rewrite v2 configuration to v3 env vars

v2 uses a ConfigMap (`kuberhealthy.yaml`). v3 uses env vars. Map your settings:

- `listenAddress` -> `KH_LISTEN_ADDRESS`
- `logLevel` -> `KH_LOG_LEVEL`
- `maxKHJobAge` -> `KH_MAX_JOB_AGE`
- `maxCheckPodAge` -> `KH_MAX_CHECK_POD_AGE`
- `maxCompletedPodCount` -> `KH_MAX_COMPLETED_POD_COUNT`
- `maxErrorPodCount` -> `KH_MAX_ERROR_POD_COUNT`
- `promMetricsConfig.suppressErrorLabel` -> `KH_PROM_SUPPRESS_ERROR_LABEL`
- `promMetricsConfig.errorLabelMaxLength` -> `KH_PROM_ERROR_LABEL_MAX_LENGTH`
- `namespace` -> `KH_TARGET_NAMESPACE`
- `externalCheckReportingURL` or `KH_EXTERNAL_REPORTING_URL` -> `KH_CHECK_REPORT_URL` (base URL only; `/check` is appended)
- v2-only settings with no v3 equivalent: `enableForceMaster`, `influx*`, `stateMetadata`.

## 3) Keep CRDs side by side

- v2 CRDs: `khchecks.comcast.github.io`, `khjobs.comcast.github.io`, `kuberhealthystates.comcast.github.io`.
- v3 CRD: `healthchecks.kuberhealthy.github.io` (v2).
- v3 only watches `HealthCheck` resources unless you deploy the legacy conversion webhook.

## 4) Rewrite check manifests

`khcheck` -> `HealthCheck`:
- `apiVersion: kuberhealthy.github.io/v2`, `kind: HealthCheck`.
- Move `spec.podSpec` to `spec.podSpec.spec`.
- Move `spec.podAnnotations` to `spec.podSpec.metadata.annotations`.
- Move `spec.podLabels` to `spec.podSpec.metadata.labels`.
- Keep `runInterval`, `timeout`, `extraAnnotations`, `extraLabels`.

`khjob` -> `HealthCheck`:
- `apiVersion: kuberhealthy.github.io/v2`, `kind: HealthCheck`.
- Add `spec.singleRunOnly: true`.
- Move the pod template into `spec.podSpec.spec` and its labels/annotations into `spec.podSpec.metadata`.

## 5) Update status and reporting endpoints

- v2 JSON status: served at `/` (root).
- v3 UI: served at `/` and JSON status moved to `/json`.
- v2 check report URL: `/externalCheckStatus`.
- v3 check report URL: `/check` (derived from `KH_CHECK_REPORT_URL`).

## 6) Apply the new `HealthCheck` resources

```sh
kubectl -n kuberhealthy-v3 apply -f <converted-check>.yaml
kubectl -n kuberhealthy-v3 get healthcheck
```

## 7) Remove v2 after cutover

- Delete old `khcheck`/`khjob` resources.
- Uninstall the old Kuberhealthy deployment.
- Remove legacy `khstate` resources and CRD once no legacy objects remain.

Optional: If you want old manifests to auto-convert during migration, you must deploy the legacy conversion webhook yourself (it is not shipped in the base manifests).
