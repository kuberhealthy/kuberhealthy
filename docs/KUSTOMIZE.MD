# Kustomize Deployment

This guide covers deploying Kuberhealthy with the kustomize manifests in `deploy/kustomize`.

## Apply the base

```sh
kubectl apply -k github.com/kuberhealthy/kuberhealthy/deploy/kustomize/base?ref=main
```
Replace `ref=main` with a release tag once tags are published.

## Use a local clone

```sh
kubectl apply -k deploy/kustomize/base
```

## Overlays

If you need a load balancer or ingress, use the overlays in `deploy/kustomize` as a starting point.

## Prometheus scraping

The base Service and pod template include Prometheus scrape annotations by default. If you use Prometheus Operator, apply the ServiceMonitor manifest:

```sh
kubectl apply -f deploy/serviceMonitor.yaml
```
