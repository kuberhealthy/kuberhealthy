# HealthCheck CRD Reference

Full anatomy of the `HealthCheck` resource (`apiVersion: kuberhealthy.github.io/v2`). Generated schema: `deploy/helm/kuberhealthy/crds/healthchecks.kuberhealthy.github.io.yaml`. For end-to-end check authoring, see [CHECK_CREATION.md](CHECK_CREATION.md).

## Resource identity

- `apiVersion`: `kuberhealthy.github.io/v2`
- `kind`: `HealthCheck`
- `scope`: namespaced
- `shortNames`: `hc`, `healthcheck`, `healthchecks`

## Mental model

A `HealthCheck` has two main parts:

- `spec`: what you want Kuberhealthy to run
- `status`: what Kuberhealthy last observed

The `spec` defines when the check runs, how long it can run, and which pod Kuberhealthy should create,
or the HTTP or TCP probe Kuberhealthy should run itself.
The `status` records the latest result and run history.

## Resource anatomy

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: HealthCheck
metadata:
  name: example-check
  namespace: kuberhealthy
spec:
  singleRunOnly: false
  runInterval: 10m
  timeout: 2m
  extraAnnotations: {}
  extraLabels: {}
  podSpec:
    metadata:
      labels: {}
      annotations: {}
    spec:
      containers:
        - name: check
          image: docker.io/curlimages/curl:8.5.0
      restartPolicy: Never
status:
  ok: true
  errors: []
  consecutiveFailures: 0
  runDuration: 123456789
  namespace: kuberhealthy
  currentUUID: ""
  lastRunUnix: 1712345678
  successCount: 1
  failureCount: 0
  lastOKUnix: 1712345678
  lastFailureUnix: 0
  additionalMetadata: ""
```

## `spec` fields

### `spec.singleRunOnly`

- Type: `bool`
- Purpose: run the check one time and do not schedule it again

### `spec.runInterval`

- Type: duration
- Purpose: set how often Kuberhealthy schedules the check
- Example values: `30s`, `5m`, `1h`

### `spec.timeout`

- Type: duration
- Purpose: set how long the check has to report before Kuberhealthy marks it failed
- Set this explicitly when porting V2 checks so you do not rely on default behavior

### `spec.extraAnnotations`

- Type: `map[string]string`
- Purpose: add annotations to checker pods created for this check

### `spec.extraLabels`

- Type: `map[string]string`
- Purpose: add labels to checker pods created for this check

### `spec.podSpec`

- Type: structured wrapper around the checker pod definition
- Purpose: define the pod that Kuberhealthy will create and run

This is the biggest V3 shape change.

In V2, `podSpec` directly held the pod spec fields.
In V3, pod fields belong under:

```yaml
spec:
  podSpec:
    spec:
```

## `spec.podSpec.metadata`

### `spec.podSpec.metadata.labels`

- Type: `map[string]string`
- Purpose: apply labels directly to the checker pod

### `spec.podSpec.metadata.annotations`

- Type: `map[string]string`
- Purpose: apply annotations directly to the checker pod

## `spec.podSpec.spec`

- Type: Kubernetes `corev1.PodSpec`
- Purpose: define the full checker pod spec

This is where you put fields such as:

- `containers`
- `initContainers`
- `serviceAccountName`
- `nodeSelector`
- `affinity`
- `tolerations`
- `restartPolicy`
- `securityContext`
- `terminationGracePeriodSeconds`

## `spec.http` and `spec.tcp`

For a simple reachability check you can describe an HTTP request or a TCP connection instead of a pod.
Kuberhealthy then runs the probe itself, from the Kuberhealthy pod, and stores the result like a checker pod report.
No checker pod or image is involved, so NetworkPolicies have to allow traffic from the Kuberhealthy pod to the target.
Only one of `podSpec`, `http` or `tcp` can be set, and the probe is bounded by `spec.timeout`.

```yaml
spec:
  runInterval: 1m
  timeout: 30s
  http:
    url: https://payments.payments.svc:8443/healthz
    method: GET
    expectedStatusCodes: [200]
    bodyContains: ok
    headers:
      X-Probe: kuberhealthy
---
spec:
  runInterval: 30s
  timeout: 10s
  tcp:
    address: postgres.db.svc:5432
```

### `spec.http`

- `url` (required): the `http://` or `https://` address to request
- `method`: HTTP method, defaults to `GET`
- `expectedStatusCodes`: status codes that count as success, defaults to any 2xx code
- `bodyContains`: a string the response body must contain
- `bodyRegex`: a regular expression the response body must match
- `headers`: headers added to the request
- `insecureSkipVerify`: skip TLS certificate verification

### `spec.tcp`

- `address` (required): the `host:port` to connect to; the check passes when the connection opens

## `status` fields

Kuberhealthy writes these fields.
Users usually read them, not set them.

### `status.ok`

- Type: `bool`
- Meaning: whether the latest check result is healthy

### `status.errors`

- Type: `[]string`
- Meaning: error messages from the latest failed run

### `status.consecutiveFailures`

- Type: `int`
- Meaning: number of failed runs in a row

### `status.runDuration`

- Type: duration-like integer value
- Meaning: duration of the last run

### `status.namespace`

- Type: `string`
- Meaning: namespace where the check ran

### `status.currentUUID`

- Type: `string`
- Meaning: UUID for the current or most recent run

### `status.lastRunUnix`

- Type: `int64`
- Meaning: Unix timestamp of the most recent scheduled run

### `status.successCount`

- Type: `int`
- Meaning: total successful runs recorded for the check

### `status.failureCount`

- Type: `int`
- Meaning: total failed runs recorded for the check

### `status.lastOKUnix`

- Type: `int64`
- Meaning: Unix timestamp of the most recent successful run

### `status.lastFailureUnix`

- Type: `int64`
- Meaning: Unix timestamp of the most recent failed run

### `status.additionalMetadata`

- Type: `string`
- Meaning: extra metadata surfaced in JSON status output

## Minimal working example

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

## Common V2 to V3 mapping

### V2

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

### V3

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

## Common mistakes

- putting pod fields directly under `spec.podSpec` instead of `spec.podSpec.spec`
- forgetting to set `timeout` explicitly when porting a V2 check
- trying to use `KuberhealthyJob` instead of `singleRunOnly: true`
- trying to read `khstate` instead of `HealthCheck.status`

## Related docs

- [CHECK_CREATION.md](CHECK_CREATION.md)
- [CHECKS_REGISTRY.md](CHECKS_REGISTRY.md)
- [HTTP_API.MD](HTTP_API.MD)
- [RUN_ONCE_CHECKS.MD](RUN_ONCE_CHECKS.MD)
- [../MIGRATING_TO_V3.md](../MIGRATING_TO_V3.md)
