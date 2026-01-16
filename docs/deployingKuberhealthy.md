# Deploying Kuberhealthy

Kuberhealthy ships Kustomize manifests in `deploy/kustomize` and a Helm chart in `deploy/helm/kuberhealthy`.

## Kustomize

```sh
kubectl apply -k deploy/kustomize/base
```

The base Service is `kuberhealthy` on port `8080`.

## Helm (v3 chart)

```sh
helm install kuberhealthy deploy/helm/kuberhealthy -n kuberhealthy --create-namespace
```

The Service is `kuberhealthy` on port `8080`.

## ArgoCD

```sh
kubectl apply -f deploy/argocd/kuberhealthy.yaml
```

## Scaling and leader election

Kuberhealthy can run multiple controller replicas with leader election enabled. Only the leader runs checks and reaps pods, while all replicas serve the UI and APIs.

- Set `KH_LEADER_ELECTION_ENABLED=true` and ensure the controller service account can read/write `coordination.k8s.io` `leases`.
- If you run more than one replica, verify your affinity/tolerations allow scheduling on available nodes. The Helm chart uses pod anti-affinity to spread replicas, which can block scheduling in single-node clusters unless overridden.

## Expose the status page

The `/json` endpoint is served from the Service. Use a cloud-specific overlay or an ingress:

```sh
# AWS
kubectl apply -k deploy/kustomize/aws-lb-controller

# GCP
kubectl apply -k deploy/kustomize/gcp-lb-controller

# Azure
kubectl apply -k deploy/kustomize/azure-lb-controller

# Ingress (on-prem or controller-managed)
kubectl apply -k deploy/kustomize/ingress
```

## Configure

All configuration is via environment variables in the deployment. See [FLAGS.MD](FLAGS.MD).

## Verify

```sh
kubectl -n kuberhealthy get pods
kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:8080 &
curl -fsS localhost:8080/metrics
```
