# Kuberhealthy Helm Chart

This repository contains the Kuberhealthy Helm chart. The chart was extracted
from `kuberhealthy/kuberhealthy/deploy/helm/kuberhealthy` so chart releases can
use independent version tags.

## Install From Source

```sh
git clone https://github.com/kuberhealthy/helm-chart.git
helm install kuberhealthy ./helm-chart/charts/kuberhealthy \
  -n kuberhealthy \
  --create-namespace
```

## Upgrade From Source

```sh
git clone https://github.com/kuberhealthy/helm-chart.git
helm upgrade kuberhealthy ./helm-chart/charts/kuberhealthy \
  -n kuberhealthy
```

## Common Settings

- Set the namespace with `--namespace` or `--create-namespace`.
- Configure environment variables in `deployment.env`.
- Adjust replicas and rollout strategy in `deployment`.
- The chart enables `KH_LEADER_ELECTION_ENABLED=true` by default so
  `deployment.replicas > 1` stays single-leader for scheduling and reaping.
- The chart defaults `service.type` to `LoadBalancer`. Use `ClusterIP` for
  clusters without a cloud load balancer.

```sh
helm upgrade kuberhealthy ./helm-chart/charts/kuberhealthy \
  -n kuberhealthy \
  --set service.type=ClusterIP
```

## Prometheus Scraping

By default, the chart adds Prometheus scrape annotations to the Service and pod
template.

Disable either set of annotations as needed:

```sh
helm upgrade kuberhealthy ./helm-chart/charts/kuberhealthy \
  -n kuberhealthy \
  --set service.prometheusScrape.enabled=false \
  --set deployment.prometheusScrape.enabled=false
```

If you use Prometheus Operator, apply the ServiceMonitor manifest from the main
Kuberhealthy repository:

```sh
kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/main/deploy/serviceMonitor.yaml
```

## Release Process

Chart releases are tagged independently from the main Kuberhealthy application
repo. To release a new chart version:

1. Update `charts/kuberhealthy/Chart.yaml`.
2. Commit the chart change.
3. Tag the chart repository with the chart version, prefixed with `v`.
4. Push the tag.

For example:

```sh
git tag v1.0.0
git push origin v1.0.0
```

The release workflow validates the chart, packages it, and attaches the chart
archive to the GitHub release for the tag.

## Migration From The Main Repository

The chart previously lived in the main repository at
`deploy/helm/kuberhealthy`. New chart changes and releases should happen in this
repository. Users installing from a local checkout should update commands from:

```sh
helm install kuberhealthy deploy/helm/kuberhealthy -n kuberhealthy --create-namespace
```

to:

```sh
helm install kuberhealthy ./helm-chart/charts/kuberhealthy -n kuberhealthy --create-namespace
```
