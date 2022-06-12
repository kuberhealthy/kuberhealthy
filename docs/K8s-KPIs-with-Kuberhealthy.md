Guide on how to install and use Kuberhealthy in order to capture some helpful synthetic [KPIs](https://kpi.org/KPI-Basics).

### Deploying Kuberhealthy

To install Kuberhealthy, make sure you have [Helm 3](https://helm.sh/docs/intro/install/) installed. If not, you can use the generated flat spec files located
in this [deploy folder](https://github.com/kuberhealthy/kuberhealthy/tree/master/deploy). You should use [kuberhealthy-prometheus.yaml](https://github.com/kuberhealthy/kuberhealthy/blob/master/deploy/kuberhealthy-prometheus.yaml) if you don't use the [Prometheus Operator](https://github.com/coreos/prometheus-operator), and [kuberhealthy-prometheus-operator.yaml](https://github.com/kuberhealthy/kuberhealthy/blob/master/deploy/kuberhealthy-prometheus-operator.yaml) if you do.  If you don't use Prometheus at all, you can still use Kuberhealthy with a JSON status page and/or InfluxDB integration using [this spec](https://github.com/kuberhealthy/kuberhealthy/blob/master/deploy/kuberhealthy.yaml).

#### To install using Helm 3:
##### 1. Create namespace "kuberhealthy" in the desired Kubernetes cluster/context:
  ```
  kubectl create namespace kuberhealthy
  ```
##### 2. Set your current namespace to "kuberhealthy":
  ```
  kubectl config set-context --current --namespace=kuberhealthy
  ```
##### 3. Add the kuberhealthy repo to Helm:
  ```
  helm repo add kuberhealthy https://kuberhealthy.github.io/kuberhealthy/helm-repos
  ```
##### 4. Depending on your Prometheus implementation, install Kuberhealthy using the appropriate command for your cluster:

  - If you use the [Prometheus Operator](https://github.com/coreos/prometheus-operator):
  ```
  helm install kuberhealthy kuberhealthy/kuberhealthy --set prometheus.enabled=true,prometheus.prometheusRule.enabled=true,prometheus.serviceMonitor.enabled=true
  ```

  - If you use Prometheus, but NOT Prometheus Operator:
  ```
  helm install kuberhealthy kuberhealthy/kuberhealthy --set prometheus.enabled=true,prometheus.prometheusRule.enabled=true
  ```
   See additional details about configuring the appropriate scrape annotations in the section [Prometheus Integration Details](#prometheus-integration-details) below.

   - Finally, if you don't use Prometheus:
  ```
  helm install kuberhealthy kuberhealthy/kuberhealthy
  ```

Running the Helm command should automatically install the newest version of Kuberhealthy (v2.3.0) along with a few basic checks. If you run `kubectl get pods`, you should see two Kuberhealthy pods. These are the pods that create, coordinate, and track test pods. These two Kuberhealthy pods also serve a JSON status page as well as a `/metrics` endpoint. Every other pod you see created is a checker pod designed to execute and shut down when done.

### Configuring Additional Checks

Next, you can run `kubectl get khchecks`. You should see three Kuberhealthy checks installed by default:
- [daemonset](https://github.com/kuberhealthy/kuberhealthy/tree/master/cmd/daemonset-check): Deploys and tears down a daemonset to ensure all nodes in the cluster are functional.
- [deployment](https://github.com/kuberhealthy/kuberhealthy/tree/master/cmd/deployment-check): Creates a deployment and then triggers a rolling update.  Tests that the deployment is reachable via a service and then deletes everything. Any problem in this process will cause this check to report a failure.
- [dns-status-internal](https://github.com/kuberhealthy/kuberhealthy/tree/master/cmd/dns-resolution-check): Validates that internal cluster DNS is functioning as expected.

To view other available checks, check out the [checks registry](https://github.com/kuberhealthy/kuberhealthy/blob/master/docs/CHECKS_REGISTRY.md) where you can find other yaml files you can apply to your cluster to enable various checks.

Kuberhealthy check pods should start running shortly after Kuberhealthy starts running (1-2 minutes). Additionally, the check-reaper cronjob runs every few minutes to ensure there are no more than 5 completed checker pods left lying around at a time.

To get status page view of these checks, you'll need to either expose the `kuberhealthy` service externally by editing the service `kuberhealthy` and setting `Type: LoadBalancer` or use `kubectl port-forward service/kuberhealthy 8080:80`. When viewed, the service endpoint will display a JSON status page that looks like this:

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
            "LastRun": "2020-04-06T23:20:31.7176964Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "9abd3ec0-b82f-44f0-b8a7-fa6709f759cd"
        },
        "kuberhealthy/deployment": {
            "OK": true,
            "Errors": [],
            "RunDuration": "29.142295647s",
            "Namespace": "kuberhealthy",
            "LastRun": "2020-04-06T23:20:31.7176964Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "5f0d2765-60c9-47e8-b2c9-8bc6e61727b2"
        },
        "kuberhealthy/dns-status-internal": {
            "OK": true,
            "Errors": [],
            "RunDuration": "2.43940936s",
            "Namespace": "kuberhealthy",
            "LastRun": "2020-04-06T23:20:44.6294547Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "c85f95cb-87e2-4ff5-b513-e02b3d25973a"
        }
    },
    "CurrentMaster": "kuberhealthy-7cf79bdc86-m78qr"
}
```

This JSON page displays all Kuberhealthy checks running in your cluster. If you have Kuberhealthy checks running in different namespaces, you can filter them by adding the `GET` variable `namespace` parameter: `?namespace=kuberhealthy,kube-system` onto the status page URL.

#### Custom Fields in Status Page

Sometimes it is desirable to add custom static data to status page to get a more enriched health report.
The extra status page fields are read and parsed from kuberhealthy.yaml config file which can bet set via helm chart values:

```yaml
stateMetadata:
  ClusterName: "my-cluster-name"
  ClusterVersion: "1.24"
```

It will create a status page with metadata:

```json
{
    "OK": true,
    // other standard fields
    "Metadata": {
      "ClusterName": "my-cluster-name",
      "ClusterVersion": "1.22"
    }
}
```
### Writing Your Own Checks

Kuberhealthy is designed to be extended with custom check containers that can be written by anyone to check anything. These checks can be written in any language as long as they are packaged in a container. This makes Kuberhealthy an excellent platform for creating your own synthetic checks!

Creating your own check is a great way to validate your client library, simulate real user workflow, and create a high level of confidence in your service or system uptime.

To learn more about writing your own checks, along with simple examples, check the [custom check creation](https://github.com/kuberhealthy/kuberhealthy/blob/master/docs/CHECK_CREATION.md) documentation.


### Prometheus Integration Details

When enabling Prometheus (not the operator), the Kuberhealthy service gets the following annotations added:
```.env
prometheus.io/path: /metrics
prometheus.io/port: "8080"
prometheus.io/scrape: "true"
```

In your prometheus configuration, add the following example scrape_config that scrapes the Kuberhealthy service given the added prometheus annotation:

```yaml
- job_name: 'kuberhealthy'
  scrape_interval: 1m
  honor_labels: true
  metrics_path: /metrics
  kubernetes_sd_configs:
  - role: service
    namespaces:
      names:
        - kuberhealthy
  relabel_configs:
    - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape]
      action: keep
      regex: true
```

You can also specify the target endpoint to be scraped using this example job:
```yaml
- job_name: kuberhealthy
  scrape_interval: 1m
  honor_labels: true
  metrics_path: /metrics
  static_configs:
    - targets:
      - kuberhealthy.kuberhealthy.svc.cluster.local:80
```

Once the appropriate prometheus configurations are applied, you should be able to see the following Kuberhealthy metrics:
- `kuberhealthy_check`
- `kuberhealthy_check_duration_seconds`
- `kuberhealthy_cluster_states`
- `kuberhealthy_running`

### Creating Key Performance Indicators

Using these Kuberhealthy metrics, our team has been able to collect KPIs based on the following definitions, calculations, and PromQL queries.

*Availability*

We define availability as the K8s cluster control plane being up and functioning as expected. This is measured by our ability to create a deployment, do a rolling update, and delete the deployment within a set period of time.

We calculate this by measuring Kuberhealthy's [deployment check](https://github.com/kuberhealthy/kuberhealthy/tree/master/cmd/deployment-check) successes and failures.
  - Availability = Uptime / (Uptime * Downtime)
  - Uptime = Number of Deployment Check Passes * Check Run Interval
  - Downtime = Number of Deployment Check Fails * Check Run Interval
  - Check Run Interval = how often the check runs (`runInterval` set in your KuberhealthyCheck Spec)

- PromQL Query (Availability % over the past 30 days):
  ```promql
  1 - (sum(count_over_time(kuberhealthy_check{check="kuberhealthy/deployment", status="0"}[30d])) OR vector(0))/(sum(count_over_time(kuberhealthy_check{check="kuberhealthy/deployment", status="1"}[30d])) * 100)
  ```

*Utilization*

We define utilization as user uptake of product (k8s) and its resources (pods, services, etc.). This is measured by how many nodes, deployments, statefulsets, persistent volumes, services, pods, and jobs are being utilized by our customers.
We calculate this by counting the total number of nodes, deployments, statefulsets, persistent volumes, services, pods, and jobs.

*Duration (Latency)*

We define duration as the control plane's capacity and utilization of throughput. We calculate this by capturing the average run duration of a Kuberhealthy [deployment check](https://github.com/kuberhealthy/kuberhealthy/tree/master/cmd/deployment-check) run.

- PromQL Query (Deployment check average run duration):
  ```promql
  avg(kuberhealthy_check_duration_seconds{check="kuberhealthy/deployment"})
  ```

*Errors / Alerts*

We define errors as all k8s cluster and Kuberhealthy related alerts. Every time one of our Kuberhealthy check fails, we are alerted of this failure.
