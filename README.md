![Kuberhealthy Logo](assets/kuberhealthy.png)

# Kuberhealthy

Kuberhealthy runs synthetic checks inside your Kubernetes cluster and exports status plus Prometheus metrics. v3 is a rewrite with a new `HealthCheck` CRD. If you are upgrading from v2, plan a clean reinstall and recreate your checks using the v3 schema.

## Quick start

1. Install Kuberhealthy:

   ```sh
   # Kustomize
   kubectl apply -k github.com/kuberhealthy/kuberhealthy/deploy/kustomize/base

   # Helm (from this repo)
   helm install kuberhealthy deploy/helm/kuberhealthy -n kuberhealthy --create-namespace

   # ArgoCD (pre-made application)
   kubectl apply -f deploy/argocd/kuberhealthy.yaml
   ```

2. Port-forward the service:

   ```sh
   # Kustomize
   kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:8080

   # Helm
   kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:8080
   ```

3. Open `http://localhost:8080` and apply a [HealthCheck](docs/CHECKS_REGISTRY.md).

## Docs

- [Deploying Kuberhealthy](docs/deployingKuberhealthy.md)
- [How Kuberhealthy Works](docs/howItWorks.md)
- [Creating a HealthCheck](docs/CHECK_CREATION.md)
- [HealthCheck Registry](docs/CHECKS_REGISTRY.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)

## Community

- Slack: `#kuberhealthy` in Kubernetes Slack
- Monthly meeting: 24th day of each month at 04:30 PM Pacific
  - [Join](https://zoom.us/j/96855374061)
  - [Calendar invite](https://zoom.us/meeting/tJIlcuyrqT8qHNWDSx3ZozYamoq2f0ruwfB0/ics?icsToken=98tyKuCupj4vGdORsB-GRowAGo_4Z-nwtilfgo1quCz9UBpceDr3O-1TYLQvAs3H)

## Contributing

We welcome early adopters and focused PRs. Start with the [contributing guide](docs/CONTRIBUTING.md) and feel free to add your org to [ADOPTERS](docs/ADOPTERS.md).
