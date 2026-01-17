# ArgoCD Application Deployment

This guide covers deploying Kuberhealthy using the prebuilt ArgoCD Application manifest.

## What you get

- An ArgoCD Application that syncs the Kuberhealthy Helm chart in this repo.
- Opinionated defaults for namespace, sync policy, and target revision.

## Apply the application

```sh
kubectl apply -f deploy/argocd/kuberhealthy.yaml
```

## Customize the application

- Change the namespace in `deploy/argocd/kuberhealthy.yaml` if you do not want `kuberhealthy`.
- Update `targetRevision` to pin a release tag or a specific commit.
- Adjust `syncPolicy` if you want manual or automated syncs.

## Prometheus scraping

The Helm chart includes Prometheus scrape annotations by default. If you use Prometheus Operator, apply the ServiceMonitor manifest:

```sh
kubectl apply -f deploy/serviceMonitor.yaml
```
