# Deploying Kuberhealthy

Kuberhealthy ships with [Kustomize](https://kustomize.io/) manifests in the `deploy/kustomize` directory. The examples below assume you cloned this repository and are working from its root.

## Deploy with Kustomize

Apply the base manifests to install Kuberhealthy:

```sh
kubectl apply -k deploy/kustomize/base
```

The base deployment exposes a Service named `kuberhealthy` on port `8080` inside the cluster.

## Deploy with Helm (v3 chart)

Install the v3 Helm chart from this repository:

```sh
helm install kuberhealthy3 deploy/helm/kuberhealthy3 -n kuberhealthy --create-namespace
```

The Helm chart exposes a Service named `kuberhealthy3` on port `8080`.

## Deploy with ArgoCD

Apply the pre-made ArgoCD Application (uses the Helm chart from this repo):

```sh
kubectl apply -f deploy/argocd/kuberhealthy3.yaml
```

This registers Kuberhealthy with ArgoCD and lets the controller reconcile its manifests.


## Exposing the Status Page

Kuberhealthy serves a JSON status page at `/status`. The following sections show how to expose that page with a cloud provider load balancer or an ingress.

### Amazon EKS

Use the overlay that patches the Service with the AWS load balancer annotations:

```sh
kubectl apply -k deploy/kustomize/aws-lb-controller
```

This creates an external AWS load balancer pointing at the Kuberhealthy Service.

### Google GKE

Use the GKE overlay to provision a GCP load balancer:

```sh
kubectl apply -k deploy/kustomize/gcp-lb-controller
```

GKE will create an external load balancer that forwards traffic to the Kuberhealthy Service.

### Azure AKS

A Kustomize overlay is provided for Azure. It marks the Service as `type: LoadBalancer` and disables the internal load balancer flag so that AKS creates an external load balancer:

```sh
kubectl apply -k deploy/kustomize/azure-lb-controller
```

### On‑Premises Clusters

If your cluster runs on‑premises, you can either expose the Service as a load balancer using your own controller (for example [MetalLB](https://metallb.universe.tf/)) or publish it through an Ingress:

```sh
kubectl apply -k deploy/kustomize/ingress
```

The ingress overlay creates a simple HTTP ingress pointing to the Kuberhealthy Service.

## Configuration

Kuberhealthy is configured entirely with environment variables. The deployment manifest in this repository includes default values for all options. Modify the container's environment variables to tune Kuberhealthy for your cluster. See [FLAGS.md](FLAGS.md) for the full list.

## Viewing Configured Checks

You can list checks that are configured with:

```sh
kubectl -n kuberhealthy get healthcheck
```

Check status can be accessed via the JSON status page endpoint or by inspecting the status field on the `healthcheck` resource.

## Verifying the Deployment

After installation, verify that Kuberhealthy is running and serving metrics:

```sh
kubectl get pods -n kuberhealthy
kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:8080 &
curl -f localhost:8080/metrics
```

The `kuberhealthy` pod should be in a `Running` state and the metrics endpoint should respond. If checks fail, consult the [troubleshooting guide](TROUBLESHOOTING.md).
