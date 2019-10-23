<center><img src="https://github.com/Comcast/kuberhealthy/blob/master/images/kuberhealthy.png?raw=true"></center><br />

An extensible [synthetic monitor](https://en.wikipedia.org/wiki/Synthetic_monitoring) operator for [Kubernetes](https://kubernetes.io).  Write your own tests in your own container and Kuberhealthy will manage everything else.  Automatically creates and sends metrics to [Prometheus](https://prometheus.io) and [InfluxDB](https://www.influxdata.com/).  Included JSON status page. Supplements other solutions like [Prometheus](https://prometheus.io/) very nicely!

[![Docker Repository on Quay](https://quay.io/repository/comcast/kuberhealthy/status "Kuberhealthy Docker Repository on Quay")](https://quay.io/repository/comcast/kuberhealthy)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/Comcast/kuberhealthy)](https://goreportcard.com/report/github.com/Comcast/kuberhealthy)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/2822/badge)](https://bestpractices.coreinfrastructure.org/projects/2822)

You can reach out to us on the [Kubernetes Slack](http://slack.k8s.io/) in the [#kuberhealthy channel](https://kubernetes.slack.com/messages/CB9G7HWTE).

## What is Kuberhealthy?

Kuberhealthy is an [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) for running synthetic checks.  You create a custom resource (`khcheck`) to enable various checks (you can write your own) and Kuberhealthy does all the work of scheduling your checks, ensuring they run, maintaining up/down state, and producing metrics.  There are [lots of useful checks available]("docs/EXTERNAL_CHECKS_REGISTRY.md") to ensure core functionality of Kubernetes.  By default, a core set of lightweight but valuable checks are included.  If you like, you can easily [write your own checks (TODO)]() in any language! 

Kuberhealthy serves a JSON status page, a [Prometheus](https://prometheus.io/) metrics endpoint, and allows for forwarding to an InfluxDB endpoint for integration into your choice of alerting solution.

Here is an illustration of how Kuberhealthy runs checks each in their own pod.  In this example, the checker pod both deploys a daemonset and tears it down while carefully watching for errors.  The result of the check is then sent back to Kuberhealthy and channeled into upstream metrics and status pages to indicate basic Kubernetes cluster functionality across all nodes in a cluster.

<img src="images/kh-ds-check.gif">


## Installation (TODO issue #189)
To install using [Helm](https://helm.sh) *without* Prometheus:
`helm install stable/kuberhealthy`

To install using [Helm](https://helm.sh) *with* Prometheus:
`helm install stable/kuberhealthy --set prometheus.enabled=true`

To install using [Helm](https://helm.sh) *with* Prometheus Operator:
`helm install stable/kuberhealthy --set prometheus.enabled=true --set prometheus.serviceMonitor=true`

To install using flat yaml spec files, see the [deploy directory](https://github.com/Comcast/kuberhealthy/tree/master/deploy).

After installation, Kuberhealthy will only be available from within the cluster (`Type: ClusterIP`) at the service URL `kuberhealthy.kuberhealthy`.  To expose Kuberhealthy to an external checking service, you must edit the service `kuberhealthy` and set `Type: LoadBalancer`.

RBAC bindings and roles are included in all configurations.

Kuberhealthy is currently tested on Kubernetes `1.9.x`, to `1.14.x`.

### Prometheus Alerts

A `ServiceMonitor` configuration is available at [deploy/servicemonitor.yaml](https://raw.githubusercontent.com/Comcast/kuberhealthy/master/deploy/servicemonitor.yaml).


### Grafana Dashboard

A `Grafana` dashboard is available at [deploy/grafana/dashboard.json](https://raw.githubusercontent.com/Comcast/kuberhealthy/master/deploy/grafana/dashboard.json).  To install this dashboard, follow the instructions [here](http://docs.grafana.org/reference/export_import/#importing-a-dashboard).

### Why Are Synthetic Tests Important?

Instead of trying to identify all the things that could potentially go wrong in your application or cluster with never-ending metrics and alert configurations, synthetic tests replicate real workflow and carefully check for the expected behavior to occur.  By default, Kuberhealthy monitors all basic Kubernetes cluster functionality including deployments, daemonsets, services, nodes, kube-system health and more.

Some examples of problems Kuberhealthy has detected in production with just the default checks enabled:

- Nodes where new pods get stuck in `Terminating` due to CNI communication failures
- Nodes where new pods get stuck in `ContainerCreating` due to disk provisoning errors
- Nodes where new pods get stuck in `Pending` due to container runtime errors
- Nodes where Docker or Kubelet crashes or has restarted
- Nodes that are unable to properly communicate with the api server due to kube-api request limiting
- Nodes that cannot provision or terminate pods quickly enough due to high I/O wait
- A pod in the `kube-system` namespace that is restarting too quickly
- A [Kubernetes component](https://kubernetes.io/docs/concepts/overview/components/) that is in a non-ready state
- Intermittent failures to access or create [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- Kubernetes system services remaining technically "healthy" while their underlying pods are crashing too much
  - kube-scheduler
  - kube-apiserver
  - kube-dns

##### Status Page

If you choose to alert from the JSON status page, you can access the status on `http://kuberhealthy.kuberhealthy`.  The status page displays server status in the format shown below.  The boolean `OK` field can be used to indicate global up/down status, while the `Errors` array will contain a list of all check error descriptions.  Granular, per-check information, including the last time a check was run, and the Kuberhealthy pod ran that specific check is available under the `CheckDetails` object.

```json
  {
  "OK": true,
  "Errors": [],
  "CheckDetails": {
    "DaemonSetChecker": {
      "OK": true,
      "Errors": [],
      "LastRun": "2018-06-21T17:31:33.845218901Z",
      "AuthoritativePod": "kuberhealthy-7cf79bdc86-m78qr"
    },
    "PodRestartChecker namespace kube-system": {
      "OK": true,
      "Errors": [],
      "LastRun": "2018-06-21T17:31:16.45395092Z",
      "AuthoritativePod": "kuberhealthy-7cf79bdc86-m78qr"
    },
    "PodStatusChecker namespace kube-system": {
      "OK": true,
      "Errors": [],
      "LastRun": "2018-06-21T17:32:16.453911089Z",
      "AuthoritativePod": "kuberhealthy-7cf79bdc86-m78qr"
    }
  },
  "CurrentMaster": "kuberhealthy-7cf79bdc86-m78qr"
}
```

#### High Availability

Kuberhealthy scales horizontally in order to be fault tolerant.  By default, two instances are used with a [pod disruption budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) and [RollingUpdate](https://kubernetes.io/docs/tasks/run-application/rolling-update-replication-controller/) strategy to ensure high availability.

##### Centralized Check State State

The state of checks is centralized as [custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) records.  This allows Kuberhealthy to always serve the same result, no matter which node in the pool you hit.  The current master running checks is calculated by all nodes in the deployment by simply querying the Kubernetes API for 'Ready' Kuberhealthy pods of the correct label, and sorting them alphabetically by name.  The node that comes first is master.


### Security Considerations

By default, Kuberhealthy exposes an insecure (non-HTTPS) JSON status endpoint without authentication. You should never expose this endpoint to the public internet. Exposing Kuberhealthy's status page to the public internet could result in private cluster information being exposed to the public internet when errors occur and are displayed on the page.

Vulnerabilities or other security related issues should be logged as git issues in this project and immediately reported to The Security Incident Response Team (SIRT) via email at NETO_SIRT@comcast.com.  Please do not post sensitive information in git issues.
