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
KH_LISTEN_ADDRESS=":8080"
KH_LOG_LEVEL="info"
KH_MAX_JOB_AGE=""
KH_MAX_CHECK_POD_AGE=""
KH_MAX_COMPLETED_POD_COUNT="0"
KH_MAX_ERROR_POD_COUNT="0"
KH_PROM_SUPPRESS_ERROR_LABEL="false"
KH_PROM_ERROR_LABEL_MAX_LENGTH="0"
KH_TARGET_NAMESPACE="kuberhealthy"
KH_DEFAULT_RUN_INTERVAL="10m"
KH_CHECK_REPORT_HOSTNAME="kuberhealthy.kuberhealthy.svc.cluster.local"
KH_TERMINATION_GRACE_PERIOD="5m"
KH_DEFAULT_CHECK_TIMEOUT="5m"
KH_DEBUG_MODE="false"
KH_DEFAULT_NAMESPACE="kuberhealthy"
```
