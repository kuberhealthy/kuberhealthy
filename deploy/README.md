### Deployment Specs

This directory contains several ways to deploy Kuberhealthy to your cluster.  You do not need all of the specs in this directory.

Each flat spec file here requires you to first create the `kuberhealthy` namespace with `kubectl create ns kuberhealthy` before application.  Then, simply use `kubectl apply -f` on the file to deploy Kuberhealthy and some basic checks into your cluster.

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

A helm chart for deploying Kuberhealthy.  This is the same helm chart published in our Helm registry.  Install this chart with the following steps:

- Create namespace "kuberhealthy" in the desired Kubernetes cluster/context:
`kubectl create namespace kuberhealthy`

- Set your current namespace to "kuberhealthy":
`kubectl config set-context --current --namespace=kuberhealthy`

- Add the kuberhealthy repo to Helm:
`helm repo add kuberhealthy https://comcast.github.io/kuberhealthy/helm-repos`

- Install kuberhealthy with Helm using one of the following:

  - Without Prometheus:
  `helm install kuberhealthy kuberhealthy/kuberhealthy`
  
  - With Prometheus:
  `helm install kuberhealthy kuberhealthy/kuberhealthy --set prometheus.enabled=true --set prometheus.enableScraping=true --set prometheus.enableAlerting=true`
 
 - With Prometheus Operator:
  `helm install kuberhealthy kuberhealthy/kuberhealthy --set prometheus.enabled=true --set prometheus.enableScraping=true --set prometheus.enableAlerting=true --set prometheus.serviceMonitor=true`


### Helm

`grafana/`

An example dashboard for displaying Kuberhealthy checks from Prometheus data sources.  Import the json file within to Grafana. 
