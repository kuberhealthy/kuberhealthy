# Building Locally

If you don't have `just` or `podman` installed, install them.

- To run tests, use `just test`
- To build locally, run `just build`
- The image will build as `kuberhealthy:localdev`
- To apply the Kubernetes specs to a cluster, run `just kustomize`
- To run the image locally, use `just run`


# Environment Variables
```
KH_TARGET_NAMESPACE="kuberhealthy"
KH_CHECK_REPORT_HOSTNAME="kuberhealthy.kuberhealthy.svc.cluster.local"
```
