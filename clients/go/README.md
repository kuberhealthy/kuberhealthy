# Example Go Client

This directory contains a minimal Go program that reports its status back to [Kuberhealthy](https://github.com/kuberhealthy/kuberhealthy). The program reads the `KH_REPORTING_URL` and `KH_RUN_UUID` environment variables injected into checker pods and sends either a success or failure result using the `checkclient` package.

## Customizing the Check

1. Edit `main.go` and add your own logic.
2. Call `checkclient.ReportSuccess()` when your check passes.
3. Call `checkclient.ReportFailure([]string{"message"})` when it fails.

## Building

```sh
make build
make docker-build IMAGE=yourrepo/example-check:tag
make docker-push IMAGE=yourrepo/example-check:tag
```

## Deploying

Create a `KHCheck` that references the image you built:

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: KHCheck
metadata:
  name: example-check
spec:
  runInterval: 1m
  timeout: 30s
  podSpec:
    containers:
    - name: example
      image: yourrepo/example-check:tag
      env:
      - name: FAIL
        value: "true"
```

Apply the resource to any cluster running Kuberhealthy:

```sh
kubectl apply -f khcheck.yaml
```
