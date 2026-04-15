# Kuberhealthy Documentation

These guides cover installing, operating, and extending Kuberhealthy. You can write your own HealthChecks in any language to validate anything, including synthetic workflow simulation (end-to-end user flows). Start with [CHECK_CREATION.md](CHECK_CREATION.md) and the client libraries below.

The web interface at `/` shows HealthCheck status at a glance, `/json` provides a machine-readable summary, and `/metrics` is ready for Prometheus.

## Client Libraries and Examples

| Language | Client |
|---|---|
| [Go](https://github.com/kuberhealthy/go) | `github.com/kuberhealthy/kuberhealthy/v3/pkg/checkclient` |
| [Python](https://github.com/kuberhealthy/python) | `kuberhealthy` |
| [TypeScript](https://github.com/kuberhealthy/typescript) | `@kuberhealthy/kuberhealthy` |
| [JavaScript](https://github.com/kuberhealthy/javascript) | `@kuberhealthy/kuberhealthy` |
| [Rust](https://github.com/kuberhealthy/rust) | `kuberhealthy` |
| [Ruby](https://github.com/kuberhealthy/ruby) | `kuberhealthy` |
| [Java](https://github.com/kuberhealthy/java) | Maven / Gradle |
| [Bash](https://github.com/kuberhealthy/bash) | Shell script helper |

## Documentation Index

| 📌 | Doc | Purpose |
| --- | --- | --- |
| 📘 | [Docs Index (this page)](README.md) | Full documentation entrypoint. |
| ⚡ | [Quickstart](QUICKSTART.MD) | Install, deploy a check, and verify. |
| 🚀 | [Deploying Kuberhealthy](DEPLOYING_KUBERHEALTHY.MD) | Deployment overview and rollout tips. |
| ⛵ | [Helm Chart](HELM.MD) | Helm install, upgrade, and scrape settings. |
| 🌐 | [ArgoCD Application](ARGOCD.MD) | ArgoCD application manifest usage. |
| 🧱 | [Kustomize Manifests](KUSTOMIZE.MD) | Base and overlay kustomize deployment. |
| 🧠 | [How Kuberhealthy Works](HOW_IT_WORKS.MD) | Operator internals and flow. |
| 🔗 | [HTTP API](HTTP_API.MD) | Endpoints for UI, checks, and automation. |
| 🧪 | [Run Once Checks](RUN_ONCE_CHECKS.MD) | One-shot validation runs. |
| 🧩 | [HealthCheck Creation](CHECK_CREATION.md) | Building custom checks. |
| ✅ | [HealthCheck Registry](CHECKS_REGISTRY.md) | Ready-to-apply check catalog. |
| 🎛️ | [Configuration](FLAGS.md) | Environment configuration variables. |
| 📈 | [Metrics Catalog](METRICS_CATALOG.MD) | Prometheus metrics and labels. |
| 🧲 | [ServiceMonitor](prometheus/SERVICE_MONITOR.MD) | Prometheus Operator ServiceMonitor guide. |
| 🧯 | [Troubleshooting](TROUBLESHOOTING.md) | Debugging steps and recovery. |
| 🏗️ | [Build and Release](BUILD_AND_RELEASE.MD) | Build, tag, and release workflow. |
| 🗒️ | [Release Notes](RELEASE_NOTES.MD) | Version changes and upgrades. |
| 🧭 | [Migrate to V3](../MIGRATING_TO_V3.md) | V2 to V3 migration guidance. |
| 🤝 | [Contributing](CONTRIBUTING.md) | Contribution workflow. |
| 🧑‍💻 | [Contributors](CONTRIBUTORS.md) | People and acknowledgements. |
| 🏢 | [Adopters](ADOPTERS.md) | Organizations using Kuberhealthy. |
| 📜 | [Code of Conduct](CODE_OF_CONDUCT.md) | Community standards. |
| 🏛️ | [Architecture](agent/ARCHITECTURE.md) | System design view. |
| 🔁 | [Logic Flow](agent/LOGIC.md) | Runtime flow and control points. |
| 🔌 | [Interfaces](agent/INTERFACES.md) | Inputs, outputs, and APIs. |
| 🧱 | [Structures](agent/STRUCTURES.md) | Key data structures. |
| ⚙️ | [Configuration](agent/CONFIGURATION.md) | Configuration details and defaults. |

## Additional References

- [OpenAPI Schema](../openapi.yaml)
- [`HealthCheck` CRD Reference](CRD_REFERENCE.MD)
- [Prometheus Rules](prometheus/PROMETHEUS_RULES.yaml)
- [ServiceMonitor Manifest](../deploy/serviceMonitor.yaml)
- [PodMonitor Example](prometheus/POD_MONITOR.yaml)
- [Prometheus Scrape Example](prometheus/PROMETHEUS_SCRAPE_CONFIG.yaml)
- [Compatibility](COMPATIBILITY.MD)
- [V2 to V3 Migration Guide](../MIGRATING_TO_V3.md)
