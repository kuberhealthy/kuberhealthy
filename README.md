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

3. Open `http://localhost:8080` and apply a [HealthCheck](docs/CHECKS_REGISTRY.md).

## Next docs to read

| 📌 | Doc | Why it matters |
| --- | --- | --- |
| 🚀 | [Deploying Kuberhealthy](docs/deployingKuberhealthy.md) | Installation patterns and rollout guidance. |
| ✅ | [HealthCheck Registry](docs/CHECKS_REGISTRY.md) | Ready-to-apply checks for common cluster signals. |
| 🧰 | [Troubleshooting](docs/TROUBLESHOOTING.md) | Debugging steps for failed checks and controller issues. |
| 🗒️ | [Release Notes](docs/releaseNotes.md) | Upgrade notes and version-specific changes. |
