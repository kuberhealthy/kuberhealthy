# Kuberhealthy ServiceMonitor

A ServiceMonitor is a Prometheus Operator custom resource that tells Prometheus how to scrape a Kubernetes Service. Use it when you run Prometheus with the Operator (for example, kube-prometheus-stack or OpenShift monitoring).

## When to use it

- You already run Prometheus Operator or kube-prometheus-stack.
- You want Prometheus to discover Kuberhealthy via Service labels instead of static scrape configs.

If you run vanilla Prometheus without the Operator, use the scrape config example instead: `docs/prometheus/PROMETHEUS_SCRAPE_CONFIG.yaml`.

## Where it lives

- Manifest: `deploy/serviceMonitor.yaml`

## How to apply

```bash
kubectl apply -f deploy/serviceMonitor.yaml
```

The ServiceMonitor expects the Kuberhealthy Service to be labeled `app: kuberhealthy` and uses the `/metrics` endpoint on port `http`.
The manifest is scoped to the `kuberhealthy` namespace by default; update `metadata.namespace` and `spec.namespaceSelector.matchNames` if you deploy elsewhere.
