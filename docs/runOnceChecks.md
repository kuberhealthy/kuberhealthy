# Run Once Checks

Run once checks are single-execution `HealthCheck` resources. They are ideal for upgrades, migrations, and one-time smoke tests.

## Create a single-run `HealthCheck`

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: HealthCheck
metadata:
  name: upgrade-smoke
  namespace: kuberhealthy
spec:
  singleRunOnly: true
  timeout: 5m
  podSpec:
    spec:
      containers:
        - name: main
          image: docker.io/kuberhealthy/kuberhealthy:main
          command:
            - sh
            - -c
            - |
              set -euo pipefail
              curl -fsS https://kubernetes.default.svc/version
              curl -fsS -H "Content-Type: application/json"                 -H "kh-run-uuid: $KH_RUN_UUID"                 -d '{"OK":true,"Errors":[]}' "$KH_REPORTING_URL"
      restartPolicy: Never
```

Apply the manifest with `kubectl apply -f upgrade-smoke.yaml`.
Replace the image with your own check image before running in production.

## Wait for the result

```sh
kubectl -n kuberhealthy wait   --for=jsonpath='{.status.lastRunUnix}'!=0   --timeout=10m   healthcheck/upgrade-smoke
```

Inspect the result via `kubectl -n kuberhealthy describe healthcheck upgrade-smoke` or the `/json` status page.

## Clean up

```sh
kubectl -n kuberhealthy delete healthcheck upgrade-smoke
```
