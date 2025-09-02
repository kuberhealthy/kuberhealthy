
![Kuberhealthy Logo](assets/kuberhealthy.png)

**Kuberhealthy is a [CNCF Sandbox Project](https://www.cncf.io/sandbox-projects/)!**

**Kuberhealthy is a [Kubernetes operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) for [synthetic monitoring](https://en.wikipedia.org/wiki/Synthetic_monitoring) and [continuous process verification](https://en.wikipedia.org/wiki/Software_verification_and_validation).**  You can [write your own test containers](docs/CHECK_CREATION.md) in any language and Kuberhealthy will run them and produce metrics for [Prometheus](https://prometheus.io). Includes a simple JSON status page for custom integrations.

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/kuberhealthy/kuberhealthy)](https://goreportcard.com/report/github.com/kuberhealthy/kuberhealthy)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/2822/badge)](https://bestpractices.coreinfrastructure.org/projects/2822)
[![Twitter Follow](https://img.shields.io/twitter/follow/kuberhealthy.svg?style=social)](https://twitter.com/kuberhealthy)  
[![Join Slack](https://img.shields.io/badge/slack-kubernetes/kuberhealthy-teal.svg?logo=slack)](https://kubernetes.slack.com/messages/CB9G7HWTE)

## What is Kuberhealthy?

Kuberhealthy lets you continuously verify that your applications and Kubernetes clusters are working as expected. By creating a custom resource (a [`KuberhealthyCheck`](https://github.com/kuberhealthy/kuberhealthy/blob/master/docs/CHECKS.md#khcheck-anatomy)) in your cluster, you can easily enable [various synthetic tests](docs/CHECKS_REGISTRY.md) and get Prometheus metrics for them.

Kuberhealthy comes with [lots of useful checks already available](docs/CHECKS_REGISTRY.md) to ensure the core functionality of Kubernetes, but checks can be used to test anything you like.  We encourage you to [write your own check container](docs/CHECK_CREATION.md) in any language to test your own applications.  It really is quick and easy!

Kuberhealthy serves the status of all checks on a simple JSON status page and a [Prometheus](https://prometheus.io/) metrics endpoint (at `/metrics`). The OpenAPI specification for the API is available as JSON at `/openapi.yaml` and `/openapi.json` on the web server.


### DaemonSet Check in Action

Kuberhealthy creating and tearing down a daemonset across the cluster:

<img src="assets/kh-ds-check.gif" alt="Daemonset check animation">



## Quick Start

1. Install Kuberhealthy into your cluster:

   ```sh
   kubectl apply -k github.com/kuberhealthy/kuberhealthy/deploy
   ```

2. Forward the Kuberhealthy service to your local machine:

   ```sh
   kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:80
   ```

3. Visit [http://localhost:8080](http://localhost:8080) to view the status page.

For advanced configuration options, see the [deployment guide](docs/deployingKuberhealthy.md) and the [full documentation](docs/).

## Client Libraries and Examples

Kuberhealthy offers example applications and importable clients for a variety of languages:

- [Rust](https://github.com/kuberhealthy/rust)
- [TypeScript](https://github.com/kuberhealthy/typescript)
- [JavaScript](https://github.com/kuberhealthy/javascript)
- [Go](https://github.com/kuberhealthy/go)
- [Python](https://github.com/kuberhealthy/python)
- [Ruby](https://github.com/kuberhealthy/ruby)
- [Java](https://github.com/kuberhealthy/java)
- [Bash](https://github.com/kuberhealthy/bash)

## Learn More

- 🧠 [How Kuberhealthy Works](docs/howItWorks.md)
- 🚀 [Deploying Kuberhealthy](docs/deployingKuberhealthy.md)
- 📊 [Viewing Check Status](docs/checkStatus.md)
- 🛠️ [Creating Your Own `khcheck`](docs/CHECK_CREATION.md)
- 🗂️ [khcheck Registry](docs/CHECKS_REGISTRY.md)

## Contributing

If you're interested in contributing to this project:
- Check out the [Contributing Guide](docs/CONTRIBUTING.md).
- If you use Kuberhealthy in a production environment, add yourself to the list of [Kuberhealthy adopters](docs/ADOPTERS.md)!
- Check out [open issues](https://github.com/kuberhealthy/kuberhealthy/issues). If you're new to the project, look for the `good first issue` tag.
- We're always looking for check contributions (either in suggestions or in PRs) as well as feedback from folks implementing
Kuberhealthy locally or in a test environment.

### Hermit

While working on Kuberhealthy, you can take advantage of the included [Hermit](https://cashapp.github.io/hermit/) dev 
environment to get Go & other tooling without having to install them separately on your local machine.

Just use the following command to activate the environment, and you're good to go:

```zsh
. ./bin/activate-hermit
```

## Monthly Community Meeting

If you would like to talk directly to the core maintainers to discuss ideas, code reviews, or other complex issues, we have a monthly Zoom meeting on the **24th day** of every month at **04:30 PM Pacific Time**.  

- [Click here to download the invite file](https://zoom.us/meeting/tJIlcuyrqT8qHNWDSx3ZozYamoq2f0ruwfB0/ics?icsToken=98tyKuCupj4vGdORsB-GRowAGo_4Z-nwtilfgo1quCz9UBpceDr3O-1TYLQvAs3H)
or
- [Click here to join the zoom meeting right now (968 5537 4061)](https://zoom.us/j/96855374061)
