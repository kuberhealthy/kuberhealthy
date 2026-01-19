# Helm Chart Deployment

This guide covers installing and operating Kuberhealthy with the Helm chart in this repo.

## What you get

- A Deployment, Service, RBAC, CRDs, and related Kubernetes resources.
- Optional Prometheus scrape annotations on the Service and pod template.

## Install

```sh
helm install kuberhealthy deploy/helm/kuberhealthy -n kuberhealthy --create-namespace
```
Pin to a release tag when chart packages are available.

## Common settings

- Set the namespace with `--namespace` or `--create-namespace` as shown above.
- Configure environment variables in `deployment.env`.
- Adjust replicas and rollout strategy in `deployment`.

## Prometheus scraping

By default, the chart adds Prometheus scrape annotations to the Service and pod template.

Disable either set of annotations as needed:

```sh
helm upgrade kuberhealthy deploy/helm/kuberhealthy \
  -n kuberhealthy \
  --set service.prometheusScrape.enabled=false \
  --set deployment.prometheusScrape.enabled=false
```

If you use Prometheus Operator, apply the ServiceMonitor manifest:

```sh
kubectl apply -f deploy/serviceMonitor.yaml
```

## Upgrade

```sh
helm upgrade kuberhealthy deploy/helm/kuberhealthy -n kuberhealthy
```

## Uninstall

```sh
helm uninstall kuberhealthy -n kuberhealthy
```
