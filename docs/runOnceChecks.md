# Run Once Checks

Sometimes you only need a single proof that things still work, like after a risky cluster upgrade or before rolling out a new admission policy. Run once checks let you use the familiar `HealthCheck` resource to launch a pod exactly one time and capture the result without additional runs.

## Craft a Single-Run `khcheck`

Start with the same manifest structure as a recurring check and add `singleRunOnly: true` to the spec. The following example runs a smoke test pod once and exits:

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: healthcheck
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
          image: curlimages/curl:8.5.0
          command:
            - sh
            - -c
            - |
              set -euo pipefail
              curl -fsS https://kubernetes.default.svc/version
              curl -fsS -H "Content-Type: application/json" \
                -H "kh-run-uuid: $KH_RUN_UUID" \
                -d '{"OK":true,"Errors":[]}' "$KH_REPORTING_URL"
      restartPolicy: Never
```

Apply the manifest with the usual `kubectl apply -f upgrade-smoke.yaml`. Because the resource type is identical, the Kuberhealthy controller handles the pod lifecycle the same way it would for any scheduled check—it just stops once the run completes.

## Wait for the First Result

You can make `kubectl` block until the first status update arrives by waiting for `status.lastRunUnix` to become non-zero:

```sh
kubectl -n kuberhealthy wait \
  --for=jsonpath='{.status.lastRunUnix}'!=0 \
  --timeout=10m \
  khcheck/mycheck
```

Replace `mycheck` with the name of your resource—`upgrade-smoke` in this example.

When the command returns, the pod has either reported success or delivered an error payload back to the controller. Inspect the detailed result with `kubectl -n kuberhealthy describe khcheck upgrade-smoke` or by port-forwarding to the `/status` page like any other check.

## Clean Up After the Run

Single-run checks stay in the cluster so you can review their status later, but they will not schedule again. Delete the resource once you have captured the outcome:

```sh
kubectl -n kuberhealthy delete khcheck upgrade-smoke
```

This pattern gives you the confidence of a full `khcheck` lifecycle with the simplicity of a one-time job—perfect for upgrade smoke tests, feature flags, or ephemeral diagnostics.
