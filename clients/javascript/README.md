# JavaScript Kuberhealthy Client

This directory contains an example external check for [Kuberhealthy](https://github.com/kuberhealthy/kuberhealthy) written in JavaScript. The script demonstrates how to report a successful run or a failure back to Kuberhealthy using environment variables provided to every checker pod.

## Usage

1. **Add your logic**: edit `check.js` and replace the placeholder section in `main` with your own check logic. Call `report(true, [])` when the check succeeds or `report(false, ["message"])` on failure.
2. **Build the image**: run `make build IMAGE=my-registry/my-check:tag` to build a container image containing your check.
3. **Push the image**: `make push IMAGE=my-registry/my-check:tag`.
4. **Create a KuberhealthyCheck**: write a khcheck resource that references your image and apply it to clusters where Kuberhealthy runs.

The check relies on two environment variables set automatically by Kuberhealthy:

- `KH_REPORTING_URL` – the endpoint where status reports are posted.
- `KH_RUN_UUID` – the UUID for this check run. It must be sent back in the `kh-run-uuid` header.

## Example khcheck

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: KuberhealthyCheck
metadata:
  name: example-js-check
  namespace: kuberhealthy
spec:
  runInterval: 1m
  podSpec:
    containers:
      - name: check
        image: my-registry/my-check:tag
```

Apply the file with `kubectl apply -f <file>`.
