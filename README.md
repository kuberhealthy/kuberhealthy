<p align="center">
  <img src="assets/kuberhealthy.png" alt="Kuberhealthy">
</p>

# Kuberhealthy

**Kuberhealthy is an operator for [synthetic monitoring](https://en.wikipedia.org/wiki/Synthetic_monitoring) and [continuous validaton](https://en.wikipedia.org/wiki/Software_verification_and_validation). It ships metrics to Prometheus and enables you to package your monitoring as Kubernetes manfiests.**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/kuberhealthy/kuberhealthy)](https://goreportcard.com/report/github.com/kuberhealthy/kuberhealthy)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/2822/badge)](https://bestpractices.coreinfrastructure.org/projects/2822)
[![Join Slack](https://img.shields.io/badge/slack-kubernetes/kuberhealthy-teal.svg?logo=slack)](https://kubernetes.slack.com/messages/CB9G7HWTE)

__Kuberhealthy provides a `HealthCheck` custom resource definition to make the running of your own custom check containers easy.__

## Getting started

1. Install Kuberhealthy to your cluster:

   Kustomize
   ```sh
   kubectl apply -k github.com/kuberhealthy/kuberhealthy/deploy/kustomize/base
   ```

   Helm (from this repo)
   ```sh
   helm install kuberhealthy deploy/helm/kuberhealthy -n kuberhealthy --create-namespace
   ```

   ArgoCD (pre-made application resource)
   ```sh
   kubectl apply -f deploy/argocd/kuberhealthy.yaml
   ```

2. Port-forward the service:

   ```sh
   kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:8080
   ```

3. Open `http://localhost:8080` and apply a [HealthCheck](docs/CHECKS_REGISTRY.MD).

## Docs table of contents

| 📌 | Doc | Purpose |
| --- | --- | --- |
| 📘 | [Docs Index](docs/README.MD) | Full documentation entrypoint. |
| 🚀 | [Deploying Kuberhealthy](docs/DEPLOYINGKUBERHEALTHY.MD) | Deployment overview and rollout tips. |
| ⛵ | [Helm Chart](docs/HELM.MD) | Helm install, upgrade, and scrape settings. |
| 🌐 | [ArgoCD Application](docs/ARGOCD.MD) | ArgoCD application manifest usage. |
| 🧱 | [Kustomize Manifests](docs/KUSTOMIZE.MD) | Base and overlay kustomize deployment. |
| 🧠 | [How Kuberhealthy Works](docs/HOWITWORKS.MD) | Operator internals and flow. |
| 🧪 | [Run Once Checks](docs/RUNONCECHECKS.MD) | One-shot validation runs. |
| 🧩 | [HealthCheck Creation](docs/CHECK_CREATION.MD) | Building custom checks. |
| ✅ | [HealthCheck Registry](docs/CHECKS_REGISTRY.MD) | Ready-to-apply check catalog. |
| 🎛️ | [Flags](docs/FLAGS.MD) | Environment configuration flags. |
| 📈 | [Metrics Catalog](docs/METRICSCATALOG.MD) | Prometheus metrics and labels. |
| 🧲 | [ServiceMonitor](docs/prometheus/SERVICEMONITOR.MD) | Prometheus Operator ServiceMonitor guide. |
| 🧯 | [Troubleshooting](docs/TROUBLESHOOTING.MD) | Debugging steps and recovery. |
| 🏗️ | [Build and Release](docs/BUILDANDRELEASE.MD) | Build, tag, and release workflow. |
| 🗒️ | [Release Notes](docs/RELEASENOTES.MD) | Version changes and upgrades. |
| 🧭 | [Migrate to HealthCheck](docs/MIGRATINGTOHEALTHCHECK.MD) | Migration guidance. |
| 🤝 | [Contributing](docs/CONTRIBUTING.MD) | Contribution workflow. |
| 🧑‍💻 | [Contributors](docs/CONTRIBUTORS.MD) | People and acknowledgements. |
| 🏢 | [Adopters](docs/ADOPTERS.MD) | Organizations using Kuberhealthy. |
| 📜 | [Code of Conduct](docs/CODE_OF_CONDUCT.MD) | Community standards. |
| 🏛️ | [Architecture](docs/agent/ARCHITECTURE.MD) | System design view. |
| 🔁 | [Logic Flow](docs/agent/LOGIC.MD) | Runtime flow and control points. |
| 🔌 | [Interfaces](docs/agent/INTERFACES.MD) | Inputs, outputs, and APIs. |
| 🧱 | [Structures](docs/agent/STRUCTURES.MD) | Key data structures. |
| ⚙️ | [Configuration](docs/agent/CONFIGURATION.MD) | Configuration details and defaults. |
