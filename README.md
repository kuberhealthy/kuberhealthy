
<center><img src="https://github.com/kuberhealthy/kuberhealthy/blob/master/images/kuberhealthy.png?raw=true"></center><br />

# ⚠️ Kuberhealthy v3 has moved to the [main branch](https://github.com/kuberhealthy/kuberhealthy/tree/main) and is no longer maintained here!

**For more info, check out the [migration guide](docs/MigrationGuide.md).


```yaml
apiVersion: kuberhealthy.github.io/v2
kind: KuberhealthyCheck
metadata:
  name: example
spec:
  runInterval: 1m
  podSpec:
    containers:
    - name: example
      image: ghcr.io/kuberhealthy/example-check:v3
```

**Kuberhealthy is a [Kubernetes](https://kubernetes.io) [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) for [synthetic monitoring](https://en.wikipedia.org/wiki/Synthetic_monitoring) and [continuous process verification](https://en.wikipedia.org/wiki/Continued_process_verification).**  [Write your own tests](docs/CHECK_CREATION.md) in any language and Kuberhealthy will run them for you.  Automatically creates metrics for [Prometheus](https://prometheus.io).  Includes simple JSON status page.  **Now part of the CNCF!**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/kuberhealthy/kuberhealthy)](https://goreportcard.com/report/github.com/kuberhealthy/kuberhealthy)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/2822/badge)](https://bestpractices.coreinfrastructure.org/projects/2822)
[![Twitter Follow](https://img.shields.io/twitter/follow/kuberhealthy.svg?style=social)](https://twitter.com/kuberhealthy)  
[![Join Slack](https://img.shields.io/badge/slack-kubernetes/kuberhealthy-teal.svg?logo=slack)](https://kubernetes.slack.com/messages/CB9G7HWTE)

## Table of Contents

- ❓ [What is Kuberhealthy?](#what-is-kuberhealthy)
- 🚀 [Installation](#installation)
- 📈 [Visualized](#visualized)
- 🧪 [Create Synthetic Checks](#create-synthetic-checks-for-your-apis)
- 📊 [Status Page](#status-page)
- 🤝 [Contributing](#contributing)
- 📅 [Monthly Community Meeting](#monthly-community-meeting)

## What is Kuberhealthy?

Kuberhealthy lets you continuously verify that your applications and Kubernetes clusters are working as expected. By creating a custom resource (a [`KuberhealthyCheck`](docs/CHECK_CREATION.md#creating-your-khcheck-resource)) in your cluster, you can easily enable [various synthetic tests](docs/CHECKS_REGISTRY.md) and get Prometheus metrics for them.

Kuberhealthy comes with [lots of useful checks already available](docs/CHECKS_REGISTRY.md) to ensure the core functionality of Kubernetes, but checks can be used to test anything you like.  We encourage you to [write your own check container](docs/CHECK_CREATION.md) in any language to test your own applications.  It really is quick and easy!

Kuberhealthy serves the status of all checks on a simple JSON status page, a [Prometheus](https://prometheus.io/) metrics endpoint (at `/metrics`), and supports InfluxDB metric forwarding for integration into your choice of alerting solution.



## Installation

Kuberhealthy requires Kubernetes 1.16 or above. You can install it with plain YAML manifests or with Helm.

- For detailed installation steps, see the [installation guide](docs/INSTALLATION.md).
- To configure Kuberhealthy after installation, see the [configuration documentation](docs/CONFIGURATION.md).

## Visualized

Here is an illustration of how Kuberhealthy provisions and operates checker pods.  The following process is illustrated:

- An admin creates a [`KuberhealthyCheck`](docs/CHECK_CREATION.md#creating-your-khcheck-resource) resource that calls for a synthetic Kubernetes daemonset to be deployed and tested every 15 minutes.  This will ensure that all nodes in the Kubernetes cluster can provision containers properly.
- Kuberhealthy observes this new `KuberhealthyCheck` resource.
- Kuberhealthy schedules a checker pod to manage the lifecycle of this check.
- The checker pod creates a daemonset using the Kubernetes API.
- The checker pod observes the daemonset and waits for all daemonset pods to become `Ready`
- The checker pod deletes the daemonset using the Kubernetes API.
- The checker pod observes the daemonset being fully cleaned up and removed.
- The checker pod reports a successful test result back to Kuberhealthy's API.
- Kuberhealthy stores this check's state and makes it available to various metrics systems.


<img src="images/kh-ds-check.gif">

## Included Checks

You can use any of [the pre-made checks](https://github.com/kuberhealthy/kuberhealthy/blob/master/docs/CHECKS_REGISTRY.md#khcheck-registry) by simply enabling them.  By default Kuberhealthy comes with several checks to test Kubernetes deployments, daemonsets, and DNS.

#### Some checks you can easily enable:

- [SSL Handshake Check](https://github.com/kuberhealthy/kuberhealthy/blob/master/cmd/ssl-handshake-check/README.md) - checks SSL certificate validity and warns when certs are about to expire.
- [CronJob Scheduling Failures](https://github.com/kuberhealthy/kuberhealthy/blob/master/cmd/cronjob-checker/README.md) - checks for events indicating that a CronJob has failed to create Job pods.
- [Image Pull Check](https://github.com/kuberhealthy/kuberhealthy/blob/master/cmd/test-check#image-pull-check) - checks that an image can be pulled from an image repository.
- [Deployment Check](https://github.com/kuberhealthy/kuberhealthy/blob/master/cmd/deployment-check/README.md) - verifies that a fresh deployment can run, deploy multiple pods, pass traffic, do a rolling update (without dropping connections), and clean up successfully.
- [Daemonset Check](https://github.com/kuberhealthy/kuberhealthy/blob/master/cmd/daemonset-check/README.md) - verifies that a daemonset can be created, fully provisioned, and torn down.  This checks the full kubelet functionality of every node in your Kubernetes cluster.
- [Storage Provisioner Check](https://github.com/ChrisHirsch/kuberhealthy-storage-check) - verifies that a pod with persistent storage can be configured on every node in your cluster.


## Create Synthetic Checks for Your APIs

You can easily create synthetic tests to check your applications and APIs with real world use cases. This is a great way to be confident that your application functions as expected in the real world at all times.

Here is a full check example written in `go`.  Just implement `doCheckStuff` and you're off!


```go
package main

import (
  "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
)

func main() {
  ok := doCheckStuff()
  if !ok {
    checkclient.ReportFailure([]string{"Test has failed!"})
    return
  }
  checkclient.ReportSuccess()
}

```

You can read more about [how checks are configured](docs/CHECK_CREATION.md#creating-your-khcheck-resource) and [learn how to create your own check container](docs/CHECK_CREATION.md). Checks can be written in any language and helpful clients for checks not written in Go can be found in the [clients directory](/clients).

### Status Page

Kuberhealthy serves a simple JSON status page and Prometheus metrics endpoint. See the [status page guide](docs/STATUS_PAGE.md) for output examples and details.

## Contributing

If you're interested in contributing to this project:
- Check out the [Contributing Guide](CONTRIBUTING.md).
 - If you use Kuberhealthy in a production environment, add yourself to the list of [Kuberhealthy adopters](ADOPTERS.md)!
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
