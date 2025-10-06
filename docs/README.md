# Kuberhealthy Documentation

Welcome to the Kuberhealthy documentation directory. These guides cover installing, using, and contributing to Kuberhealthy.

## Client Libraries and Examples

Kuberhealthy provides example applications and importable clients for multiple languages:

- [Rust](https://github.com/kuberhealthy/rust)
- [TypeScript](https://github.com/kuberhealthy/typescript)
- [JavaScript](https://github.com/kuberhealthy/javascript)
- [Go](https://github.com/kuberhealthy/go)
- [Python](https://github.com/kuberhealthy/python)
- [Ruby](https://github.com/kuberhealthy/ruby)
- [Java](https://github.com/kuberhealthy/java)
- [Bash](https://github.com/kuberhealthy/bash)

## Table of Contents

- 🚀 [Deploying Kuberhealthy](deployingKuberhealthy.md): Install with Kustomize or ArgoCD and expose the `/status` page with load balancers or an ingress.
- 📊 [Viewing Kuberhealthy Check Status](howItWorks.md#using-the-json-status-page): Reach the `/status` endpoint and inspect `khcheck` status fields.
- 🧠 [How Kuberhealthy Works](howItWorks.md): Illustration of the check lifecycle and controller interaction.
- 🕒 [Run Once Checks](runOnceChecks.md): Launch a `khcheck` for a single validation run and wait for the result.
- 🛠️ [Creating Your Own `khcheck`](CHECK_CREATION.md): Build custom checks and craft `HealthCheck` resources.
- 🗂️ [khcheck Registry](CHECKS_REGISTRY.md): Discover ready-made checks contributed by the community.
- 🚩 [Flags](FLAGS.md): Reference of command-line flags supported by Kuberhealthy.
- 🐞 [Troubleshooting](TROUBLESHOOTING.md): Solutions to common issues.
- 🏗️ [Build and Release Process](buildAndRelease.md): Automated image builds and cutting new releases.
- 🤝 [Contributing](CONTRIBUTING.md): Guidelines for contributing to the project.
- 📜 [Code of Conduct](CODE_OF_CONDUCT.md): Expected community behavior.
- 👥 [Contributors](CONTRIBUTORS.md): Individuals who have contributed to Kuberhealthy.
- 🏢 [Adopters](ADOPTERS.md): Organizations using Kuberhealthy in production.
