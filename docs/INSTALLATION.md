# Installation

## Deployment

Kuberhealthy requires Kubernetes 1.16 or above.

### Using Plain Ole' YAML

If you just want the rendered default specs without Helm, you can [use the static flat file](https://github.com/kuberhealthy/kuberhealthy/blob/master/deploy/kuberhealthy.yaml) or the [static flat file for Prometheus](https://github.com/kuberhealthy/kuberhealthy/blob/master/deploy/kuberhealthy-prometheus.yaml) or even the [static flat file for Prometheus Operator](https://github.com/kuberhealthy/kuberhealthy/blob/master/deploy/kuberhealthy-prometheus-operator.yaml).

Here are the one-line installation commands for those same specs:
```sh
# If you don't use Prometheus:
kubectl create namespace kuberhealthy
kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/master/deploy/kuberhealthy.yaml

# If you use Prometheus, but not with Prometheus Operator:
kubectl create namespace kuberhealthy
kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/master/deploy/kuberhealthy-prometheus.yaml

# If you use Prometheus Operator:
kubectl create namespace kuberhealthy
kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/master/deploy/kuberhealthy-prometheus-operator.yaml
```

### Using Helm

```sh
kubectl create namespace kuberhealthy
helm repo add kuberhealthy https://kuberhealthy.github.io/kuberhealthy/helm-repos
helm install -n kuberhealthy kuberhealthy kuberhealthy/kuberhealthy
```

If you have Prometheus

```
helm install --set prometheus.enabled=true -n kuberhealthy kuberhealthy kuberhealthy/kuberhealthy
```

If you have Prometheus via Prometheus Operator:

```
helm install --set prometheus.enabled=true --set prometheus.serviceMonitor.enabled=true -n kuberhealthy kuberhealthy kuberhealthy/kuberhealthy
```

### Configure Service

After installation, Kuberhealthy will only be available from within the cluster (`Type: ClusterIP`) at the service URL `kuberhealthy.kuberhealthy`. To expose Kuberhealthy to clients outside of the cluster, you **must** edit the service `kuberhealthy` and set `Type: LoadBalancer` or otherwise expose the service yourself.

### Edit Configuration Settings

You can edit the Kuberhealthy configmap as well and it will be automatically reloaded by Kuberhealthy. All configmap options are set to their defaults to make configuration easy.

`kubectl edit -n kuberhealthy configmap kuberhealthy`

### See Configured Checks

You can see checks that are configured with `kubectl -n kuberhealthy get khcheck`. Check status can be accessed by the JSON status page endpoint, or via `kubectl -n kuberhealthy get khstate`.

## Further Configuration

To configure Kuberhealthy after installation, see the [configuration documentation](CONFIGURATION.md).

Details on using the helm chart are [documented here](../deploy/helm/kuberhealthy). The Helm installation of Kuberhealthy is automatically updated to use the latest [Kuberhealthy release](https://github.com/kuberhealthy/kuberhealthy/releases).

More installation options, including static yaml files are available in the [deploy](../deploy) directory. These flat spec files contain the most recent changes to Kuberhealthy, or the master branch. Use this if you would like to test master branch updates.

