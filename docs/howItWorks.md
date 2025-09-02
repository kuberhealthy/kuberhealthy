# How Kuberhealthy Works

Here is an illustration of how Kuberhealthy provisions and operates checker pods. The following process is illustrated:

- An admin creates a [`KuberhealthyCheck`](CHECKS.md#khcheck-anatomy) resource that calls for a synthetic Kubernetes daemonset to be deployed and tested every 15 minutes. This will ensure that all nodes in the Kubernetes cluster can provision containers properly.
- Kuberhealthy observes this new `KuberhealthyCheck` resource.
- Kuberhealthy schedules a checker pod to manage the lifecycle of this check.
- The checker pod creates a daemonset using the Kubernetes API.
- The checker pod observes the daemonset and waits for all daemonset pods to become `Ready`.
- The checker pod deletes the daemonset using the Kubernetes API.
- The checker pod observes the daemonset being fully cleaned up and removed.
- The checker pod reports a successful test result back to Kuberhealthy's API.
- Kuberhealthy stores this check's state and makes it available to various metrics systems.

<img src="../assets/kh-ds-check.gif" alt="Kuberhealthy daemonset check illustration" />

