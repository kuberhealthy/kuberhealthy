# TypeScript Example Check

This directory contains a minimal TypeScript example for writing a [Kuberhealthy](https://github.com/kuberhealthy/kuberhealthy) external check. The script reads the `KH_REPORTING_URL` and `KH_RUN_UUID` environment variables set on checker pods and reports the result back to Kuberhealthy.

## Add your logic

1. Edit `src/example.ts` and replace the placeholder section with your own health check logic.
2. Call `reportSuccess()` on success or `reportFailure()` with an array of error messages when the check fails.

## Build

```sh
npm install
npm run build
```

Run the compiled check locally by providing the required environment variables:

```sh
KH_RUN_UUID=123 KH_REPORTING_URL=http://kuberhealthy.example/externalCheckStatus node dist/example.js
```

## Docker image

A simple `Dockerfile` and `Makefile` are provided.

```sh
make docker-build IMAGE=myrepo/kuberhealthy-typescript-example:latest
make docker-push IMAGE=myrepo/kuberhealthy-typescript-example:latest
```

## Deploy with Kuberhealthy

Create a `KuberhealthyCheck` that uses your image:

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: KuberhealthyCheck
metadata:
  name: typescript-example
spec:
  runInterval: 1m
  timeout: 30s
  podSpec:
    containers:
      - name: check
        image: myrepo/kuberhealthy-typescript-example:latest
```

Apply the resource to a cluster running Kuberhealthy. The checker pod will execute the script and report its status.
