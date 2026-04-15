# Kustomize Deployment

For full Kustomize deployment details, overlays, and configuration options, see [DEPLOYING_KUBERHEALTHY.MD](DEPLOYING_KUBERHEALTHY.MD).

## Quick reference

Remote apply (no local clone needed):

```sh
kubectl apply -k github.com/kuberhealthy/kuberhealthy/deploy/kustomize/base?ref=main
```

Local apply (from repo root):

```sh
kubectl apply -k deploy/kustomize/base
```

Overlays are in `deploy/kustomize/overlays/` and `deploy/kustomize/{aws,gcp,azure}-lb-controller/`. See [DEPLOYING_KUBERHEALTHY.MD](DEPLOYING_KUBERHEALTHY.MD) for when to use each.
