<p align="center">
  <img src="assets/kuberhealthy.png" alt="Kuberhealthy">
</p>

# Kuberhealthy

**Kuberhealthy is an operator for [synthetic monitoring](https://en.wikipedia.org/wiki/Synthetic_monitoring) and [continuous validation](https://en.wikipedia.org/wiki/Software_verification_and_validation). It ships metrics to Prometheus and enables you to package your synthetic monitoring as Kubernetes manifests.**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/kuberhealthy/kuberhealthy)](https://goreportcard.com/report/github.com/kuberhealthy/kuberhealthy)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/2822/badge)](https://bestpractices.coreinfrastructure.org/projects/2822)
[![CI](https://github.com/kuberhealthy/kuberhealthy/actions/workflows/tests.yml/badge.svg)](https://github.com/kuberhealthy/kuberhealthy/actions/workflows/tests.yml)
[![GitHub Release](https://img.shields.io/github/v/release/kuberhealthy/kuberhealthy)](https://github.com/kuberhealthy/kuberhealthy/releases)
[![Join Slack](https://img.shields.io/badge/slack-kubernetes/kuberhealthy-teal.svg?logo=slack)](https://kubernetes.slack.com/messages/CB9G7HWTE)


## Why Kuberhealthy?

Kuberhealthy schedules, tracks, monitors, and manages Kubernetes pods for checks — which means your monitoring logic can:

- **Run multi-step workflows** — simulate a real user: log in, create a record, verify it, clean it up
- **Use any language** — [Go](https://github.com/kuberhealthy/go), [Python](https://github.com/kuberhealthy/python), [Rust](https://github.com/kuberhealthy/rust), [bash](https://github.com/kuberhealthy/bash), or anything that fits in a container
- **Own your checks as code** — checks are Kubernetes manifests, so ship them alongside your app!


## How it works

```mermaid
graph TB
    prometheus["Prometheus /metrics"]
    browser["Web Interface"]

    subgraph cluster["Kubernetes Cluster"]
        svc["Kuberhealthy Service :80"]
        crds["HealthCheck CRDs"]
        controller["Kuberhealthy Controller"]
        pod["Check Pod\n(api-smoke-test)"]

        svc --> controller
        controller -->|watches| crds
        controller -->|schedules| pod
        pod -->|"POST /check"| controller
    end

    prometheus --> svc
    browser --> svc
```

Kuberhealthy provides the `HealthCheck` custom resource definition. Each `HealthCheck` tells Kuberhealthy to start a short-lived checker pod on a schedule. 

The checker pod runs your validation logic, then reports back to Kuberhealthy. Results flow to the built-in status UI, JSON API, and Prometheus metrics endpoint.


## Getting started

Installing Kuberhealthy is easy. Just apply the kustomize, ArgoCD, or Helm manifests and you're ready to start applying `healthcheck` resources.

1. Install Kuberhealthy to your cluster:

   **Helm** (recommended)
   ```sh
   helm install kuberhealthy deploy/helm/kuberhealthy -n kuberhealthy --create-namespace
   ```

   **Kustomize**
   ```sh
   kubectl apply -k github.com/kuberhealthy/kuberhealthy/deploy/kustomize/base?ref=main
   ```

   **ArgoCD**
   ```sh
   kubectl apply -f deploy/argocd/kuberhealthy.yaml
   ```

2. Port-forward the service:

   ```sh
   kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:80
   ```

3. Open `http://localhost:8080` to see the status UI, then apply a [HealthCheck](docs/CHECKS_REGISTRY.md) or build [your own](docs/CHECK_CREATION.md).


## What a `healthcheck` CRD looks like

This is the core object Kuberhealthy manages. It tells the controller what checker container to run, how often to run it, and how long to wait before considering the run failed. You can use `kubectl get healthcheck` or `kubectl get hc`.

This example uses the built-in [deployment check](https://github.com/kuberhealthy/deployment-check), which creates a test deployment, rolls it out, and tears it down on a schedule:

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: HealthCheck
metadata:
  name: deployment
  namespace: kuberhealthy
spec:
  runInterval: 10m
  timeout: 5m
  podSpec:
    spec:
      containers:
        - name: deployment
          image: docker.io/kuberhealthy/deployment-check:v0.1.1
          env:
            - name: CHECK_DEPLOYMENT_REPLICAS
              value: "4"
            - name: CHECK_DEPLOYMENT_ROLLING_UPDATE
              value: "true"
          resources:
            requests:
              cpu: 25m
              memory: 15Mi
            limits:
              cpu: "1"
      serviceAccountName: deployment-sa
```

See [CHECK_CREATION.md](docs/CHECK_CREATION.md) for the environment variables injected into every check pod and how to build your own.


## Example Healthcheck Use

Apply your `healthcheck` check:

```
> kubectl apply -f api-smoke-test.yaml
healthcheck.kuberhealthy.github.io/api-smoke-test.yaml created

> kubectl get healthcheck
NAME              NAMESPACE      LAST RUN    AGE    OK
api-smoke-test    kuberhealthy   2m ago      2d     true
```

Consume Check Status with Prometheus:

**`/metrics`** (Prometheus)
```
kuberhealthy_check{check="api-smoke-test",namespace="kuberhealthy",status="1"} 1
kuberhealthy_check{check="db-connectivity",namespace="kuberhealthy",status="0"} 1
kuberhealthy_check_duration_seconds{check="api-smoke-test",namespace="kuberhealthy"} 0.23
kuberhealthy_check_success_total{check="api-smoke-test",namespace="kuberhealthy"} 142
```

Fetch the status by API:

**`/json`**
```json
{
  "ok": true,
  "checks": {
    "kuberhealthy/api-smoke-test": {
      "ok": true,
      "errors": [],
      "lastRun": "2024-01-15T14:32:01Z",
      "runDuration": "230ms"
    }
  }
}
```


## Writing checks

Get started with [CHECK_CREATION.md](docs/CHECK_CREATION.md) and the [HealthCheck registry](docs/CHECKS_REGISTRY.md), then pick a check client for your language:

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

**Example: Go check that validates an internal API**

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


## Documentation

See the full documentation index in [docs/README.md](docs/README.md).


## Adopters

See [docs/ADOPTERS.md](docs/ADOPTERS.md) for organizations running Kuberhealthy in production.


## Contributing

- Read the [Contributing Guide](docs/CONTRIBUTING.md) before opening a PR.
- Browse [open issues](https://github.com/kuberhealthy/kuberhealthy/issues) — new contributors should look for the `good first issue` tag.
- Check contributions are especially welcome — see the [HealthCheck registry](docs/CHECKS_REGISTRY.md) for gaps.
- Have feedback from running Kuberhealthy in production? Open a discussion or join Slack.


## Community

- **Slack**: [`#kuberhealthy`](https://kubernetes.slack.com/messages/CB9G7HWTE) in the Kubernetes Slack workspace
- **Monthly call**: Every **24th** of the month at **4:30 PM Pacific** — [download invite](https://zoom.us/meeting/tJIlcuyrqT8qHNWDSx3ZozYamoq2f0ruwfB0/ics?icsToken=98tyKuCupj4vGdORsB-GRowAGo_4Z-nwtilfgo1quCz9UBpceDr3O-1TYLQvAs3H) · [join now](https://zoom.us/j/96855374061)
- **Issues**: [github.com/kuberhealthy/kuberhealthy/issues](https://github.com/kuberhealthy/kuberhealthy/issues)
