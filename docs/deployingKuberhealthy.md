# Deploying Kuberhealthy

Kuberhealthy ships with [Kustomize](https://kustomize.io/) manifests in the `deploy` directory. The examples below assume you cloned this repository and are working from its root.

## Deploy with Kustomize

Apply the base manifests to install Kuberhealthy:

```sh
kubectl apply -k deploy/base
```

The base deployment exposes a Service named `kuberhealthy` on port `80` inside the cluster.

## Deploy with ArgoCD

Create an ArgoCD Application to manage Kuberhealthy:

```sh
kubectl apply -k deploy/argocd
```

This registers Kuberhealthy with ArgoCD and lets the controller reconcile its manifests.


## Exposing the Status Page

Kuberhealthy serves a JSON status page at `/status`. The following sections show how to expose that page with a cloud provider load balancer or an ingress.

### Amazon EKS

Use the overlay that patches the Service with the AWS load balancer annotations:

```sh
kubectl apply -k deploy/aws-lb-controller
```

This creates an external AWS load balancer pointing at the Kuberhealthy Service.

### Google GKE

Use the GKE overlay to provision a GCP load balancer:

```sh
kubectl apply -k deploy/gcp-lb-controller
```

GKE will create an external load balancer that forwards traffic to the Kuberhealthy Service.

### Azure AKS

A Kustomize overlay is provided for Azure. It marks the Service as `type: LoadBalancer` and disables the internal load balancer flag so that AKS creates an external load balancer:

```sh
kubectl apply -k deploy/azure-lb-controller
```

### On‑Premises Clusters

If your cluster runs on‑premises, you can either expose the Service as a load balancer using your own controller (for example [MetalLB](https://metallb.universe.tf/)) or publish it through an Ingress:

```sh
kubectl apply -k deploy/ingress
```

The ingress overlay creates a simple HTTP ingress pointing to the Kuberhealthy Service.

## Configuration

Kuberhealthy is configured entirely with environment variables. The deployment manifest in this repository includes default values for all options. Modify the container's environment variables to tune Kuberhealthy for your cluster. See [FLAGS.md](FLAGS.md) for the full list.

## Viewing Configured Checks

You can list checks that are configured with:

```sh
kubectl -n kuberhealthy get khcheck
```

Check status can be accessed via the JSON status page endpoint or by inspecting the status field on the `KuberhealthyCheck` resource.

## Verifying the Deployment

After installation, verify that Kuberhealthy is running and serving metrics:

```sh
kubectl get pods -n kuberhealthy
kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:80 &
curl -f localhost:8080/metrics
```

The `kuberhealthy` pod should be in a `Running` state and the metrics endpoint should respond. If checks fail, consult the [troubleshooting guide](TROUBLESHOOTING.md).
