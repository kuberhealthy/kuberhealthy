<p align="center">
  <img src="assets/kuberhealthy.png" alt="Kuberhealthy">
</p>

# Kuberhealthy

**Kuberhealthy is an operator for [synthetic monitoring](https://en.wikipedia.org/wiki/Synthetic_monitoring) and [continuous validation](https://en.wikipedia.org/wiki/Software_verification_and_validation). It ships metrics to Prometheus and enables you to package your monitoring as Kubernetes manifests.**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/kuberhealthy/kuberhealthy)](https://goreportcard.com/report/github.com/kuberhealthy/kuberhealthy)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/2822/badge)](https://bestpractices.coreinfrastructure.org/projects/2822)
[![Join Slack](https://img.shields.io/badge/slack-kubernetes/kuberhealthy-teal.svg?logo=slack)](https://kubernetes.slack.com/messages/CB9G7HWTE)

__Kuberhealthy provides the `HealthCheck` custom resource definition (try `kubectl get healthcheck`) to make running your own check containers easy.__
The built-in web status UI (`/`) gives an at-a-glance view of HealthCheck state, with JSON summaries (`/json`) and Prometheus metrics (`/metrics`) for automation.

## Getting started

1. Install Kuberhealthy to your cluster:

   Kustomize
   ```sh
   kubectl apply -k github.com/kuberhealthy/kuberhealthy/deploy/kustomize/base?ref=main
   ```
   Tracks the latest `main` commit. Update to a release tag once tags are published.

   Helm (from this repo)
   ```sh
   helm install kuberhealthy deploy/helm/kuberhealthy -n kuberhealthy --create-namespace
   ```
   Pin to a release tag when chart packages are available.

   ArgoCD (pre-made application resource)
   ```sh
   kubectl apply -f deploy/argocd/kuberhealthy.yaml
   ```

2. Port-forward the service:

   ```sh
   kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:80
   ```

3. Open `http://localhost:8080` to see the status UI, then apply a [HealthCheck](docs/CHECKS_REGISTRY.MD) or build your own (see [CHECK_CREATION.MD](docs/CHECK_CREATION.MD)).

## Documentation

See the full documentation index in [docs/README.MD](docs/README.MD).

## Create Synthetic Checks for Your APIs

Custom HealthChecks let you validate real workflows end-to-end, catch regressions before users do, and turn runbooks into always-on synthetic verification. Checks can test anything (including multi-step synthetic workflow simulation) and can be written in any language.

Get started with [CHECK_CREATION.MD](docs/CHECK_CREATION.MD) and the [HealthCheck registry](docs/CHECKS_REGISTRY.MD), then pick a check client for your language:

- [Go](https://github.com/kuberhealthy/go)
- [Rust](https://github.com/kuberhealthy/rust)
- [Bash](https://github.com/kuberhealthy/bash)
- [Python](https://github.com/kuberhealthy/python)
- [Ruby](https://github.com/kuberhealthy/ruby)
- [JavaScript](https://github.com/kuberhealthy/javascript)
- [TypeScript](https://github.com/kuberhealthy/typescript)
- [Java](https://github.com/kuberhealthy/java)

Here is a full check example written in Go. Implement `doCheckStuff` and you are off:

```go
package main

import "github.com/kuberhealthy/kuberhealthy/v3/pkg/checkclient"

func main() {
  ok := doCheckStuff()
  if !ok {
    checkclient.ReportFailure([]string{"Test has failed!"})
    return
  }
  checkclient.ReportSuccess()
}
```

You can read more about [how checks are configured](docs/CHECK_CREATION.MD#create-the-healthcheck-resource) and [learn how to create your own check container](docs/CHECK_CREATION.MD). Checks can be written in any language and helpful clients are listed above.

## Contributing

If you are interested in contributing to this project:

- Check out the [Contributing Guide](docs/CONTRIBUTING.MD).
- If you use Kuberhealthy in production, add yourself to the list of [Kuberhealthy adopters](docs/ADOPTERS.MD).
- Check out [open issues](https://github.com/kuberhealthy/kuberhealthy/issues). If you are new to the project, look for the `good first issue` tag.
- We are always looking for check contributions and feedback from folks running Kuberhealthy locally or in production.

## Monthly Community Meeting

If you would like to talk directly to the core maintainers to discuss ideas, code reviews, or other complex issues, we have a monthly Zoom meeting on the **24th day** of every month at **04:30 PM Pacific Time**.

- [Click here to download the invite file](https://zoom.us/meeting/tJIlcuyrqT8qHNWDSx3ZozYamoq2f0ruwfB0/ics?icsToken=98tyKuCupj4vGdORsB-GRowAGo_4Z-nwtilfgo1quCz9UBpceDr3O-1TYLQvAs3H)
- [Click here to join the zoom meeting right now (968 5537 4061)](https://zoom.us/j/96855374061)
