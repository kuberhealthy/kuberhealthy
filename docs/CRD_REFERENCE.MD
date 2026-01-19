# HealthCheck CRD Reference

This document summarizes the `HealthCheck` custom resource used by Kuberhealthy v3.
For the full schema, see `deploy/helm/kuberhealthy/crds/healthchecks.kuberhealthy.github.io.yaml`.

## Resource identity

- `apiVersion`: `kuberhealthy.github.io/v2`
- `kind`: `HealthCheck`
- `scope`: Namespaced
- `shortNames`: `hc`, `healthcheck`, `healthchecks`

## Spec fields

- `spec.singleRunOnly` (bool): Run the check once and never schedule again.
- `spec.runInterval` (duration): How often to schedule the check (for example `10m`).
- `spec.timeout` (duration): Max time allowed for the check to report.
- `spec.extraAnnotations` (map[string]string): Applied to all checker pods.
- `spec.extraLabels` (map[string]string): Applied to all checker pods.
- `spec.podSpec.metadata.labels` (map[string]string): Labels for the checker pod.
- `spec.podSpec.metadata.annotations` (map[string]string): Annotations for the checker pod.
- `spec.podSpec.spec` (corev1.PodSpec): Full pod spec executed for the check.

## Status fields

- `status.ok` (bool): Current health of the check.
- `status.errors` ([]string): Latest error messages.
- `status.consecutiveFailures` (int): Sequential failure count.
- `status.runDuration` (duration): Duration of the last run.
- `status.namespace` (string): Namespace the check ran in.
- `status.currentUUID` (string): Current run UUID.
- `status.lastRunUnix` (int64): Last scheduled run time.
- `status.successCount` (int): Total successful runs.
- `status.failureCount` (int): Total failed runs.
- `status.lastOKUnix` (int64): Last OK timestamp.
- `status.lastFailureUnix` (int64): Last failure timestamp.
- `status.additionalMetadata` (string): Extra metadata shown in JSON status output.

## Minimal example

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: HealthCheck
metadata:
  name: example-check
spec:
  runInterval: 10m
  timeout: 2m
  podSpec:
    spec:
      containers:
        - name: check
          image: docker.io/curlimages/curl:8.5.0
          imagePullPolicy: IfNotPresent
          command:
            - sh
            - -c
            - |
              set -euo pipefail
              curl -fsS \
                -H "Content-Type: application/json" \
                -H "kh-run-uuid: $KH_RUN_UUID" \
                -d '{"ok":true,"errors":[]}' "$KH_REPORTING_URL"
      restartPolicy: Never
```
