# Creating Your Own `HealthCheck`

This guide walks you through building a custom check container and wiring it into Kuberhealthy.

## Client libraries

- [Rust](https://github.com/kuberhealthy/rust)
- [TypeScript](https://github.com/kuberhealthy/typescript)
- [JavaScript](https://github.com/kuberhealthy/javascript)
- [Go](https://github.com/kuberhealthy/go)
- [Python](https://github.com/kuberhealthy/python)
- [Ruby](https://github.com/kuberhealthy/ruby)
- [Java](https://github.com/kuberhealthy/java)
- [Bash](https://github.com/kuberhealthy/bash)

## Go client

Use the Go check client for report wiring:

```go
package main

import "github.com/kuberhealthy/kuberhealthy/v4/pkg/checks/external/checkclient"

func main() {
  ok := doCheckStuff()
  if !ok {
    checkclient.ReportFailure([]string{"Test has failed!"})
    return
  }
  checkclient.ReportSuccess()
}
```

## Reporting from any language

Your checker must:

- Read `KH_REPORTING_URL`.
- POST JSON to that URL with `OK` and `Errors`.
- Report before `KH_CHECK_RUN_DEADLINE`.

```json
{
  "Errors": ["Error 1 here"],
  "OK": false
}
```

Do not send `OK: true` with a non-empty `Errors` array.

### Injected environment variables

Every checker pod receives:

- `KH_REPORTING_URL`
- `KH_CHECK_RUN_DEADLINE`
- `KH_RUN_UUID` (send as header `kh-run-uuid`)
- `KH_POD_NAMESPACE`

## Create the `HealthCheck` resource

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: HealthCheck
metadata:
  name: kh-test-check
spec:
  runInterval: 30s
  timeout: 2m
  podSpec:
    containers:
    - env:
        - name: MY_OPTION_ENV_VAR
          value: "option_setting_here"
      image: docker.io/kuberhealthy/kuberhealthy:main
      imagePullPolicy: Always
      name: main
```

Replace the image with your own check image when you are ready. Once applied, Kuberhealthy schedules and reports this check like any other.

## Share your check

Add your check to the [registry](CHECKS_REGISTRY.MD) in a small PR.
