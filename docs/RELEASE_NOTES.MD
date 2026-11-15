# Release Notes

## v3.1.0

Changes since [v3.0.33](https://github.com/kuberhealthy/kuberhealthy/releases/tag/v3.0.33).

### Added

- Built-in HTTP and TCP HealthChecks run directly from the Kuberhealthy controller without creating checker pods. HTTP probes support status-code expectations, headers, and response substring or regular-expression matching. (#1558, #1559)
- HTTPS probes can enforce a minimum remaining validity for every served certificate with `spec.http.minCertificateValidity`. Negative thresholds are rejected. (#1560)
- Guides and example manifests for built-in checks and Argo Rollouts/Kargo analysis. The JSON analysis example requires the expected healthy check and namespace, preventing empty results from approving a rollout. (#1556, #1559)

### Fixed

- Report retries resend the complete request body after transport failures. (#1547; fixes #1540)
- Each report attempt has a bounded timeout so an unresponsive controller cannot block retries indefinitely. (#1553)
- Status updates retry actual Kubernetes resource conflicts with a short backoff instead of matching error text and waiting for minutes. (#1561)
- Kubernetes API client-side throttling is disabled unless configured, restoring the behavior before v3.0.23. When only QPS is configured, the client uses its default burst allowance of 10. (#1552; addresses #1551)
- Stopping or deleting a built-in check cancels its active probe. UUID-scoped cleanup preserves replacement runs and their start guards.
- Built-in probe deadlines include start bookkeeping, and results arriving after the recorded deadline cannot report success or emit success events.

### Dependencies

- Updated `devalue` from 5.8.1 to 5.9.4. (#1539)
- Updated `source-map-js` from 1.2.1 to 1.2.2. (#1542)

### Upgrade notes

- Update the HealthCheck CRD before using `spec.http`, `spec.tcp`, or `minCertificateValidity`. Existing pod-based checks remain supported; set exactly one of `podSpec`, `http`, or `tcp`.
- Built-in probes originate from the Kuberhealthy pod. Check NetworkPolicies, DNS names, and proxy settings accordingly. Header values are plaintext resource data; use a pod check with a Secret for credentials.
- Operators who want client-side API throttling should set `KH_KUBE_API_QPS` and optionally `KH_KUBE_API_BURST`; leaving both unset disables the limiter.
- This release publishes `docker.io/kuberhealthy/kuberhealthy:v3.1.0` for Linux AMD64 and ARM64 and updates the stable `latest` tag. It does not publish a new version of the separately maintained Helm chart. Explicitly select the release image when upgrading.
- Go checks importing `pkg/checkclient` must update their dependency and rebuild their checker images to receive the report-delivery fixes.

```sh
kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/v3.1.0/deploy/kustomize/base/healthcheck.yaml
```

See [BUILT_IN_CHECKS.MD](BUILT_IN_CHECKS.MD) for configuration and examples.

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
  --version 1.0.2 \
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
