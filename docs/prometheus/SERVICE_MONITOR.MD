# Kuberhealthy ServiceMonitor

Use when running Prometheus Operator (kube-prometheus-stack, OpenShift monitoring, etc.). For vanilla Prometheus, use [PROMETHEUS_SCRAPE_CONFIG.yaml](PROMETHEUS_SCRAPE_CONFIG.yaml) instead.

Manifest: `deploy/serviceMonitor.yaml`

## Apply

```bash
kubectl apply -f deploy/serviceMonitor.yaml
```

The ServiceMonitor expects the Kuberhealthy Service to be labeled `app: kuberhealthy` and uses the `/metrics` endpoint on port `http`.
The manifest is scoped to the `kuberhealthy` namespace by default; update `metadata.namespace` and `spec.namespaceSelector.matchNames` if you deploy elsewhere.
