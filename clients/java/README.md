# Java Client Example

This directory shows how to write a minimal Java check for [Kuberhealthy](https://github.com/kuberhealthy/kuberhealthy). The `ExampleCheck` program reads the `KH_REPORTING_URL` and `KH_RUN_UUID` environment variables that Kuberhealthy injects into checker pods and posts a JSON status report.

## Extending the check

Add your own logic inside `ExampleCheck.java` before the call to `report`. The file defaults to reporting success and includes commented-out lines that show how to report a failure:

```java
// ok = false;
// error = "something went wrong";
```

Call `report` with `ok=true` when the check succeeds or `ok=false` with an error message when it fails.

`report` expects an HTTP 200 response from Kuberhealthy; any other status
code causes the program to exit with an error so rejected payloads surface
immediately.

## Building and pushing

Build and push the container image:

```bash
make build IMAGE=registry.example.com/java-example:latest
make push IMAGE=registry.example.com/java-example:latest
```

## Deploying

Create a `KuberhealthyCheck` that uses the pushed image:

```yaml
apiVersion: khcheck.kuberhealthy.io/v1
kind: KuberhealthyCheck
metadata:
  name: java-example
spec:
  runInterval: 15m
  timeout: 5m
  podSpec:
    containers:
    - name: checker
      image: registry.example.com/java-example:latest
      imagePullPolicy: Always
```

Apply the manifest to a cluster running Kuberhealthy:

```bash
kubectl apply -f khcheck.yaml
```

The pod will receive the required `KH_REPORTING_URL` and `KH_RUN_UUID` variables and the check will report success or failure back to Kuberhealthy.
