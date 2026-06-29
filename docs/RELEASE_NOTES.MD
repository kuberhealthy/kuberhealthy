# Release Notes

## v3 highlights

- Leader election is available for multi-replica controller deployments. Only the leader runs checks and reaps pods, while all replicas serve the UI and APIs.
- The shipped Kustomize base and Helm chart enable leader election by default so the default `replicas: 2` deployment does not race controller scheduling.
- Leader election is configured with `KH_LEADER_ELECTION_*` environment variables and requires Lease RBAC (`coordination.k8s.io` `leases`).
- Built-in checks (daemonset, deployment, and others) live in their own repositories under the `kuberhealthy` GitHub organization.
- Prometheus metrics include controller health, run counters, and a versioned catalog in [METRICS_CATALOG.MD](METRICS_CATALOG.MD).

## Helm chart repository

- Independent Helm chart releases are published from [`kuberhealthy/kuberhealthy-helm`](https://github.com/kuberhealthy/kuberhealthy-helm).
- The main Kuberhealthy repository keeps `deploy/helm/kuberhealthy` as a source install path and compatibility reference.
- Existing Helm releases can move from the in-tree chart to the dedicated chart
  repository without changing the deployed workload by keeping the same release
  name, namespace, user values, and deployed image. See
  [HELM_CHART_REPOSITORY_MIGRATION.md](HELM_CHART_REPOSITORY_MIGRATION.md).
- The in-tree chart is not a wrapper chart. Source-tree install and upgrade
  commands that reference `deploy/helm/kuberhealthy` render the in-tree chart.

Use this migration path for an existing release:

```sh
RELEASE=kuberhealthy
NAMESPACE=kuberhealthy
DEPLOYMENT=kuberhealthy

helm get values "${RELEASE}" -n "${NAMESPACE}" --all -o yaml > kuberhealthy-values.yaml
KH_IMAGE="$(kubectl get deployment "${DEPLOYMENT}" -n "${NAMESPACE}" \
  -o jsonpath='{.spec.template.spec.containers[?(@.name=="kuberhealthy")].image}')"

helm repo add kuberhealthy https://raw.githubusercontent.com/kuberhealthy/kuberhealthy-helm/main
helm repo update

helm upgrade "${RELEASE}" kuberhealthy/kuberhealthy \
  --version 1.0.1 \
  -n "${NAMESPACE}" \
  --reuse-values \
  -f kuberhealthy-values.yaml \
  --set imageURL="${KH_IMAGE}"

helm status "${RELEASE}" -n "${NAMESPACE}"
kubectl rollout status deployment/"${DEPLOYMENT}" -n "${NAMESPACE}"
```

## Upgrade notes

- v3 uses the `HealthCheck` CRD. Use a clean reinstall and recreate checks
  with the v3 schema.
