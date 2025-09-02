# Deploying Kuberhealthy

Kuberhealthy ships with [Kustomize](https://kustomize.io/) manifests in the `deploy` directory. The examples below assume you cloned this repository and are working from its root.

## Deploy with Kustomize

Apply the base manifests to install Kuberhealthy:

```sh
kubectl apply -k deploy/base
```

The base deployment exposes a Service named `kuberhealthy` on port `80` inside the cluster.

### TLS for the Conversion Webhook

Kuberhealthy can serve its admission webhook over HTTPS when a certificate and
key are provided. Mount a secret named `kuberhealthy-webhook-tls` containing
`tls.crt` and `tls.key` into the pod or enable the optional
[`cert-manager`](../deploy/cert-manager) overlay to automatically provision a
self-signed certificate. The deployment reads the certificate from
`/tls/tls.crt` and the key from `/tls/tls.key` via the `KH_TLS_CERT_FILE` and
`KH_TLS_KEY_FILE` environment variables. If the certificate is missing or
invalid, Kuberhealthy falls back to plain HTTP and the webhook is skipped. The
mutating webhook configuration disables TLS validation by default so the API
server accepts the self-signed certificate.

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
