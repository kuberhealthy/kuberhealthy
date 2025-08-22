# Building Locally
- If you don't have `just` or `podman` installed, install them. 
- The `Justfile` from the root of the repository will be found if you run `just` from this directory. Use `just -l` to list all commands.

## Just Commands
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
