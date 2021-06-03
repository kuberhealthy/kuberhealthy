### Deployment Specs

This directory contains several ways to deploy Kuberhealthy to your cluster.  You do not need all of the specs in this directory.  Older releases are available by browsing through the release tags on Github, then checking the files in this directory at that tagged revision.

Each flat spec file here requires you to first create the `kuberhealthy` namespace with `kubectl create ns kuberhealthy` before application.  Then, simply use `kubectl apply -f` on the file to deploy Kuberhealthy and some basic checks into your cluster.  These flat spec files are automatically updated to install the most recent changes to Kuberhealthy, or everything currently in the master branch.  Use this to test the latest changes to Kuberhealthy.

### Prometheus Operator

`kuberhealthy-prometheus-operator.yaml` 

A flat file that includes everything but a namespace for clusters with Prometheus Operator installed.  For deploying Kuberhealthy in a cluster that has the [Prometheus Operator](https://github.com/coreos/prometheus-operator) deployed to it.

`servicemonitor.yaml`

A [service monitor](https://github.com/coreos/prometheus-operator#customresourcedefinitions) definition for Prometheus Operator that targets Kuberhealthy.

### Prometheus (Normal Single Instance Deployment)

`kuberhealthy-prometheus.yaml`

A flat file that includes everything but a namespace for clusters with Prometheus installed.  Create the Kuberhealthy namespace with `kubectl create ns kuberhealthy` and install with `kubectl apply -f kuberhealthy-prometheus.yaml`


### Non-Prometheus Clusters

`kuberhealthy.yaml`

A flat file that includes everything but a namespace for clusters *without* Prometheus installed.  Create the Kuberhealthy namespace with `kubectl create ns kuberhealthy` and install with `kubectl apply -f kuberhealthy.yaml`


### Helm

`helm/kuberhealthy`

A helm chart for deploying Kuberhealthy.  This is the same helm chart published in our Helm registry.  The helm chart installs the latest [Kuberhealthy release](https://github.com/kuberhealthy/kuberhealthy/releases). Install this chart with the following steps:

- Create namespace "kuberhealthy" in the desired Kubernetes cluster/context:
`kubectl create namespace kuberhealthy`

- Set your current namespace to "kuberhealthy":
`kubectl config set-context --current --namespace=kuberhealthy`

- Add the kuberhealthy repo to Helm:
`helm repo add kuberhealthy https://kuberhealthy.github.io/kuberhealthy/helm-repos`

- Install kuberhealthy with Helm using one of the following:

  - Without Prometheus:
  `helm install kuberhealthy kuberhealthy/kuberhealthy`

  - With Prometheus:
  `helm install kuberhealthy kuberhealthy/kuberhealthy --set prometheus.enabled=true  --set prometheus.prometheusRule.enabled=true`

  - With Prometheus Operator:
  `helm install kuberhealthy kuberhealthy/kuberhealthy --set prometheus.enabled=true  --set prometheus.prometheusRule.enabled=true --set prometheus.prometheusRule.release={prometheus-operator-release-name} --set prometheus.prometheusRule.namespace={prometheus-operator-namespace} --set prometheus.serviceMonitor.enabled=true --set prometheus.serviceMonitor.release={prometheus-operator-release-name} --set prometheus.serviceMonitor.namespace={prometheus-operator-namespace}` 

  - With [kube-prometheus](https://github.com/prometheus-operator/kube-prometheus):
  
    - Make sure to set your serviceMonitor and prometheusRule `release:` label with the matching serviceMonitorSelector `release:` label in your prometheus configuration. This is set to whatever you name your release when you helm install the kube-prometheus-stack. Ex. `helm install prometheus prometheus-community/kube-prometheus-stack` implies that your release name is `prometheus` and so your serviceMonitor should be set with the label `release: prometheus`.
    - Make sure you set your serviceMonitor and prometheusRule namespaces to wherever you've deployed your kube-prometheus-stack
  
    - `helm install kuberhealthy kuberhealthy/kuberhealthy --set prometheus.enabled=true  --set prometheus.prometheusRule.enabled=true --set prometheus.prometheusRule.release={prometheus-operator-release-name} --set prometheus.prometheusRule.namespace={prometheus-operator-namespace} --set prometheus.serviceMonitor.enabled=true --set prometheus.serviceMonitor.release={kube-prometheus-stack-release-name} --set prometheus.serviceMonitor.namespace={kube-prometheus-stack-namespace}`


### Helm

`grafana/`

An example dashboard for displaying Kuberhealthy checks from Prometheus data sources.  Import the json file within to Grafana. 
