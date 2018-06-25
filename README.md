# kuberhealthy

Synthetic end-to-end checks for Kubernetes clusters.

[![Docker Repository on Quay](https://quay.io/repository/comcast/kuberhealthy/status "Kuberhealthy Docker Repository on Quay")](https://quay.io/repository/comcast/kuberhealthy)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## What is Kuberhealthy?

Kuberhealthy performs operations in Kubernetes clusters just as users do in order to catch issues that would otherwise go unnoticed until they cause problems.  Kuberhealthy takes a different approach to monitoring than traditional metrics-based solutions.  Instead of trying to identify all the things that could go wrong, Kuberhealthy replicates real workflow and watches for unexpected behavior.  Kuberhealthy is not a replacement for metric-based monitoring systems such as [Prometheus](https://prometheus.io/), but it does greatly enhance metric-based monitoring by removing potential blind spots.  *Kuberhealthy is not an alerting system*.  Alerts are up to the user.

Some examples of operations that may sneak under the radar of metrics-only monitoring, but not Kuberhealthy:

- Pods stuck in `Terminating` due to CNI communication failures
- Pods stuck in `ContainerCreating` due to disk scheduler errors
- Pods stuck in `Pending` due to Docker daemon errors
- Kubernetes system services remaining technically "healthy" while their underlying pods are crashing

#### Status Page

Deploying Kuberhealthy is as simple as applying the `kubernetes.yaml` spec file in this repository and setting your systems to check the JSON on the Kuberhealthy service endpoint.  The status page displays server status in the following format.  The `OK` field can be used to indicate up/down status, while the `Errors` array will contain a list of error descriptions if any are found.  The Kuberhealthy status page provides a total cluster overview status, as well as more granular, per-check information, including the last time a check was run, and the Kuberhealthy pod that ran that specific check.

```json
  {
  "OK": true,
  "Errors": [],
  "CheckDetails": {
    "ComponentStatusChecker": {
      "OK": true,
      "Errors": [],
      "LastRun": "2018-06-21T17:32:16.921733843Z",
      "AuthorativePod": "kuberhealthy-7cf79bdc86-m78qr"
    },
    "DaemonSetChecker": {
      "OK": true,
      "Errors": [],
      "LastRun": "2018-06-21T17:31:33.845218901Z",
      "AuthorativePod": "kuberhealthy-7cf79bdc86-m78qr"
    },
    "PodRestartChecker namespace kube-system": {
      "OK": true,
      "Errors": [],
      "LastRun": "2018-06-21T17:31:16.45395092Z",
      "AuthorativePod": "kuberhealthy-7cf79bdc86-m78qr"
    },
    "PodStatusChecker namespace kube-system": {
      "OK": true,
      "Errors": [],
      "LastRun": "2018-06-21T17:32:16.453911089Z",
      "AuthorativePod": "kuberhealthy-7cf79bdc86-m78qr"
    }
  },
  "CurrentMaster": "kuberhealthy-7cf79bdc86-m78qr"
}
```

#### Centralized State

Kuberhealthy scales horizontally to be fault tolerant.  By default, two instances are used with a [pod disruption budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) and [RollingUpdate](https://kubernetes.io/docs/tasks/run-application/rolling-update-replication-controller/) strategy to ensure high availability.  The current master is calculated by all nodes in the deployment by simply querying the Kubernetes API for 'Ready' Kuberhealthy pods, and sorting them by name.  The state is centralized as [custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) records for each check.

## Checks

Kuberhealthy performs the following checks in parallel at all times:


#### daemonSet

  - Default Timeout: 5 minutes
  - Default Interval: 15 minutes

    `daemonSet` deploys a daemonSet to the `kuberhealthy` namespace, waits for all pods to be in the 'Ready' state, then terminates them and ensures all pod terminations were successful.  Containers are deployed with their resource requirements set to 0 cores and 0 memory and use the pause container from Google (`gcr.io/google_containers/pause:0.8.0`).  The `node-role.kubernetes.io/master` `NoSchedule` taint is tolerated by daemonset testing pods.  The pause container is already used by Kubelet to do various tasks and should be cached at all times.  If a failure occurs anywhere in the daemonset deployment or tear down, an error is shown on the status page describing the issue.

#### componentStatus

- Default timeout: 1 minute
- Default interval: 2 minute
- Default downtime toleration: 5 minutes

  `componentStatus` checks for the state of cluster `componentstatuses`.  Kubernetes components include the ETCD and ETCD-event deployments, the Kubernetes scheduler, and the Kubernetes controller manager.  This is almost the same as running `kubectl get componentstatuses`.  If a `componentstatus` status is down for 5 minutes, an alert is shown on the status page.

#### podRestarts

  - Default timeout: 3 minutes
  - Default interval: 5 minutes
  - Default tolerated restarts per pod over 1 hour: 5

    `podRestarts` checks for excessive pod restarts in the `kube-system` namespace.  If a pod has restarted more than five times in an hour, an error is indicated on the status page.  The exact pod's name will be shown as one of the `Error` field's strings.

    A command line flag exists `--podCheckNamespaces` which can optionally contain a comma-separated list of namespaces on which to run the podRestarts checks.  The default value is `kube-system`.  Each namespace for which the check is configured will require the `get` and `list` verbs on the `pods` resource within that namespace.

#### podStatus

  - Default timeout: 1 minutes
  - Default interval: 2 minutes
  - Default downtime toleration: 5 minutes

    `podStatus` checks for pods older than ten minutes in the `kube-system` namespace that are in an incorrect lifecycle phase (anything that is not 'Ready').  If a `podStatus` detects a pod down for 5 minutes, an alert is shown on the status page. When a pod is found to be in error, the exact pod's name will be shown as one of the `Error` field's strings.

    A command line flag exists `--podCheckNamespaces` which can optionally contain a comma-separated list of namespaces on which to run the podStatus checks.  The default value is `kube-system`.  Each namespace for which the check is configured will require the `get` and `list` verbs on the `pods` resource within that namespace.

##### Security Considerations
Kuberhealthy exposes an inscure (non-HTTPS) endpoint without authentication. You should never expose this endpoint to the public internet. Exposing Kuberhealthy to the internet could result in private cluster information being exposed to the public internet when errors occur.
