# Creating Your Own `HealthCheck`

This guide walks you through building a custom check container and wiring it into Kuberhealthy. You can validate anything — from simple HTTP probes to multi-step synthetic workflows that simulate real user behavior. Checks can be written in any language.

## How reporting works

Every check pod receives these environment variables from the controller:

| Variable | Description |
|---|---|
| `KH_REPORTING_URL` | POST your result here (full URL including `/check`) |
| `KH_RUN_UUID` | Send as the `kh-run-uuid` request header to authenticate the report |
| `KH_CHECK_RUN_DEADLINE` | Unix timestamp — your check must report before this time |
| `KH_POD_NAMESPACE` | Namespace the check pod is running in |

Your check must POST a JSON payload to `KH_REPORTING_URL` before `KH_CHECK_RUN_DEADLINE`:

```json
{
  "ok": true,
  "errors": []
}
```

Do not send `ok: true` with a non-empty `errors` array.

## Step 1: Write Your Custom Healthcheck

### Client libraries

Use a client library to handle reporting, deadline enforcement, and header wiring automatically:

| Language | Client |
|---|---|
| [Go](https://github.com/kuberhealthy/go) | `github.com/kuberhealthy/kuberhealthy/v3/pkg/checkclient` |
| [Python](https://github.com/kuberhealthy/python) | `kuberhealthy` |
| [TypeScript](https://github.com/kuberhealthy/typescript) | `@kuberhealthy/kuberhealthy` |
| [JavaScript](https://github.com/kuberhealthy/javascript) | `@kuberhealthy/kuberhealthy` |
| [Rust](https://github.com/kuberhealthy/rust) | `kuberhealthy` |
| [Ruby](https://github.com/kuberhealthy/ruby) | `kuberhealthy` |
| [Java](https://github.com/kuberhealthy/java) | Maven / Gradle |
| [Bash](https://github.com/kuberhealthy/bash) | Shell script helper |

### Example: Go check that validates an internal API

```go
package main

import (
    "fmt"
    "net/http"

    "github.com/kuberhealthy/kuberhealthy/v3/pkg/checkclient"
)

func main() {
    resp, err := http.Get("http://my-api.default.svc.cluster.local/health")
    if err != nil {
        checkclient.ReportFailure([]string{fmt.Sprintf("request failed: %s", err)})
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        checkclient.ReportFailure([]string{fmt.Sprintf("unexpected status %d from /health", resp.StatusCode)})
        return
    }

    checkclient.ReportSuccess()
}
```

The check client handles `KH_REPORTING_URL`, `KH_RUN_UUID`, and deadline enforcement automatically.

## Step 2: Build and Push Your Check Image

Push your check image to some container host that is available to your cluster.

## Step 3: Create the `HealthCheck` resource

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: HealthCheck
metadata:
  name: kh-test-check
spec:
  runInterval: 30s
  timeout: 2m
  podSpec:
    spec:
      containers:
        - name: main
          image: YOUR_CONTAINER_IMAGE:v1.0.0
          imagePullPolicy: IfNotPresent
          env:
            - name: MY_OPTION_ENV_VAR
              value: "option_setting_here"
      restartPolicy: Never
```

Replace `YOUR_CONTAINER_IMAGE` with your own check image when you are ready. Once applied, Kuberhealthy schedules and reports this check like any other.

## Share your check

Add your check to the [registry](CHECKS_REGISTRY.md) in a small PR.
