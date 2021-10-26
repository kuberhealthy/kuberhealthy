
<center><img src="https://github.com/kuberhealthy/kuberhealthy/blob/master/images/kuberhealthy.png?raw=true"></center><br />

An operator for [synthetic monitoring](https://en.wikipedia.org/wiki/Synthetic_monitoring) on [Kubernetes](https://kubernetes.io).  Write your own tests in your own container and Kuberhealthy will manage everything else.  Automatically creates and sends metrics to [Prometheus](https://prometheus.io) and [InfluxDB](https://www.influxdata.com/).  Included simple JSON status page. Supplements other solutions like [Prometheus](https://prometheus.io/) very nicely!

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/kuberhealthy/kuberhealthy)](https://goreportcard.com/report/github.com/kuberhealthy/kuberhealthy)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/2822/badge)](https://bestpractices.coreinfrastructure.org/projects/2822)
[![Twitter Follow](https://img.shields.io/twitter/follow/kuberhealthy.svg?style=social)](https://twitter.com/kuberhealthy)  
[![Join Slack](https://img.shields.io/badge/slack-kubernetes/kuberhealthy-teal.svg?logo=slack)](https://kubernetes.slack.com/messages/CB9G7HWTE)

## What is Kuberhealthy?

Kuberhealthy is an [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) for running synthetic checks.  By creating a custom resource (a `khcheck`) in your cluster, you can easily enable various synthetic test containers.  Kuberhealthy does all the work of scheduling your checks on an interval you specify (like a CronJob), ensuring they run properly within an allotted timeout, maintaining the current up/down state with durability, and producing metrics.  There are [lots of useful checks already available](docs/CHECKS_REGISTRY.md) to ensure the core functionality of Kubernetes, but checks can be used to test anything you like.  We encourage you to [write your own check container](docs/CHECK_CREATION.md) in any language to test your own applications!

Kuberhealthy serves a simple JSON status page, a [Prometheus](https://prometheus.io/) metrics endpoint (at `/metrics`), and supports InfluxDB metric forwarding for integration into your choice of alerting solution.

Here is an illustration of how Kuberhealthy provisions and operates checker pods.  In this example, the checker pod both deploys a daemonset and tears it down while carefully watching for errors.  The result of the check is then sent back to Kuberhealthy and channeled into upstream metrics and status pages to indicate basic Kubernetes cluster functionality across all nodes in a cluster.

<img src="images/kh-ds-check.gif">

## Create Synthetic Checks for Your App

With Kuberhealthy, you can easily create synthetic tests to check your applications with real world use cases.  Read more about how checks are configured in the documentation [here](docs/CHECKS.md) and learn how to create your own check container in any language [here](docs/CHECK_CREATION.md). Clients for checks outside of Go can be found in the [clients directory](/clients).


## Installation

**Requires Kubernetes 1.16 or above and Helm 3**

1. Create namespace "kuberhealthy" in the desired Kubernetes cluster/context:  
	`kubectl create namespace kuberhealthy`
2. Set your current namespace to "kuberhealthy":  
	`kubectl config set-context --current --namespace=kuberhealthy`
3. Add the kuberhealthy repo to Helm:  
	`helm repo add kuberhealthy https://kuberhealthy.github.io/kuberhealthy/helm-repos`
4. Install kuberhealthy:  
	`helm install kuberhealthy kuberhealthy/kuberhealthy`

After installation, Kuberhealthy will only be available from within the cluster (`Type: ClusterIP`) at the service URL `kuberhealthy.kuberhealthy`.  To expose Kuberhealthy to an external checking agent, you **must** edit the service `kuberhealthy` and set `Type: LoadBalancer`.  This is done for security.  Options are available in the Helm chart to bypass this and deploy with `Type: LoadBalancer` directly.

Kuberhealthy is currently tested on Kubernetes `1.22.x`.

To configure Kuberhealthy after installation, see the [configuration documentation](https://github.com/kuberhealthy/kuberhealthy/blob/master/docs/CONFIGURATION.md).

Details on using the helm chart are [documented here](https://github.com/kuberhealthy/kuberhealthy/tree/master/deploy/helm/kuberhealthy).  The Helm installation of Kuberhealthy is automatically updated to use the latest [Kuberhealthy release](https://github.com/kuberhealthy/kuberhealthy/releases).

More installation options, including static yaml files are available in the [/deploy](/deploy) directory. These flat spec files contain the most recent changes to Kuberhealthy, or the master branch. Use this if you would like to test master branch updates.

### Why Are Synthetic Tests Important?

Instead of trying to identify all the things that could potentially go wrong in your application or cluster with never-ending metrics and alert configurations, synthetic tests replicate real workflow and carefully check for the expected behavior to occur.  By default, Kuberhealthy monitors all basic Kubernetes cluster functionality including deployments, daemonsets, services, nodes, kube-system health and more.

Some examples of problems Kuberhealthy has detected in production with just the default checks enabled:

- Nodes where new pods get stuck in `Terminating` due to CNI communication failures
- Nodes where new pods get stuck in `ContainerCreating` due to disk provisoning errors
- Nodes where new pods get stuck in `Pending` due to container runtime errors
- Nodes where Docker or Kubelet is in a bad state but passing health checks
- Nodes that are unable to properly communicate with the api server due to kube-api request limiting
- Nodes that cannot provision or terminate pods quickly enough (15m) due to high I/O wait
- A pod in the `kube-system` namespace that has begun restarting too quickly
- An unexpected admission controller failure causing pod creation failure
- Intermittent failures to access or create [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- kube-dns/CoreDNS DNS lookup failures (internal and external)
- ... more!

### Status Page

You can directly access the current test statuses by accessing the `kuberhealthy.kuberhealthy` HTTP service on port 80.  The status page displays server status in the format shown below.  The boolean `OK` field can be used to indicate global up/down status, while the `Errors` array will contain a list of all check error descriptions.  Granular, per-check information, including how long the check took to run (Run Duration), the last time a check was run, and the Kuberhealthy pod ran that specific check is available under the `CheckDetails` object.

```json
{
    "OK": true,
    "Errors": [],
    "CheckDetails": {
        "kuberhealthy/daemonset": {
            "OK": true,
            "Errors": [],
            "RunDuration": "22.512278967s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:24:16.7718171Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "9abd3ec0-b82f-44f0-b8a7-fa6709f759cd"
        },
        "kuberhealthy/deployment": {
            "OK": true,
            "Errors": [],
            "RunDuration": "29.142295647s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:26:40.7444659Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "5f0d2765-60c9-47e8-b2c9-8bc6e61727b2"
        },
        "kuberhealthy/dns-status-internal": {
            "OK": true,
            "Errors": [],
            "RunDuration": "2.43940936s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:34:04.8927434Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "c85f95cb-87e2-4ff5-b513-e02b3d25973a"
        },
        "kuberhealthy/pod-restarts": {
            "OK": true,
            "Errors": [],
            "RunDuration": "2.979083775s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:34:06.1938491Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "a718b969-421c-47a8-a379-106d234ad9d8"
        }
    },
    "CurrentMaster": "kuberhealthy-7cf79bdc86-m78qr"
}
```

### High Availability

Kuberhealthy scales horizontally in order to be fault tolerant.  By default, two instances are used with a [pod disruption budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) and [RollingUpdate](https://kubernetes.io/docs/tasks/run-application/rolling-update-replication-controller/) strategy to ensure high availability.

##### Centralized Check State State

The state of checks is centralized as [custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) records.  This allows Kuberhealthy to always serve the same result, no matter which node in the pool you hit.  The current master running checks is calculated by all nodes in the deployment by simply querying the Kubernetes API for 'Ready' Kuberhealthy pods of the correct label, and sorting them alphabetically by name.  The node that comes first is master.  These two strategies together enable Kuberhealthy to maintain state and scale horizontally without deploying an additional backing database.

### Synthetic KPIs with Kuberhealthy

Using Kuberhealthy with prometheus can help capture useful synthetic KPIs. Check out the [K8s KPIs with Kuberhealthy](docs/K8s-KPIs-with-Kuberhealthy.md) doc to learn more on how to install Kuberhealthy and collect cluster KPIs.  

### Security Considerations

By default, Kuberhealthy exposes an insecure (non-HTTPS) JSON status endpoint without authentication. You should never expose this endpoint to the public internet. Exposing Kuberhealthy's status page to the public internet could result in private cluster information being exposed to the public internet when errors occur and are displayed on the page.

Vulnerabilities or other security related issues should be logged as Github issues in this project.  All new issues are reviewed regularly.  Please be careful not to post any sensitive information in your report!


## Contributing

If you're interested in contributing to this project:
- Check out the [Contributing Guide](CONTRIBUTING.md).
- If you use Kuberhealthy in a production environment, add yourself to the list of [Kuberhealthy adopters](docs/KUBERHEALTHY_ADOPTERS.md)!
- Check out [open issues](https://github.com/kuberhealthy/kuberhealthy/issues). If you're new to the project, look for the `good first issue` tag.
- We're always looking for check contributions (either in suggestions or in PRs) as well as feedback from folks implementing
Kuberhealthy locally or in a test environment.


## Monthly Community Meeting

If you would like to talk directly to the core maintainers to discuss ideas, code reviews, or other complex issues, we have a monthly Zoom meeting on the first Wednesday of the month.  [Click here to add the meeting to your calendar](https://zoom.us/j/96457488866?pwd=SDZxL1dEQTVZUTRWbFFTZWNDZWFwdz09).
