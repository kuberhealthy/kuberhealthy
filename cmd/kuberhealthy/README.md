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
KH_LISTEN_ADDRESS=":8080" # web server listen address
KH_LISTEN_ADDRESS_TLS=":443" # HTTPS listen address when TLS is enabled
KH_LOG_LEVEL="info" # log verbosity
KH_MAX_JOB_AGE="" # max age for check jobs
KH_MAX_CHECK_POD_AGE="" # max age for check pods
KH_MAX_COMPLETED_POD_COUNT="1" # completed pods to retain
KH_MAX_ERROR_POD_COUNT="2" # errored pods to retain for debugging
KH_ERROR_POD_RETENTION_TIME="36h" # duration to keep failed pods
KH_PROM_SUPPRESS_ERROR_LABEL="false" # omit error label in metrics
KH_PROM_ERROR_LABEL_MAX_LENGTH="0" # max length for error label
KH_PROM_LABEL_ALLOWLIST="severity,category" # extra metric labels to include
KH_PROM_LABEL_DENYLIST="" # extra metric labels to exclude
KH_PROM_LABEL_VALUE_MAX_LENGTH="256" # max length for extra label values
KH_TARGET_NAMESPACE="" # namespace to watch for checks; leave empty for all
POD_NAMESPACE="kuberhealthy" # namespace Kuberhealthy runs in
POD_NAME="kuberhealthy-0" # name of the running controller pod
KH_CHECK_REPORT_URL="http://kuberhealthy.kuberhealthy.svc.cluster.local" # base URL used for reports (no path)
KH_DEFAULT_RUN_INTERVAL="10m" # default check run interval
KH_TERMINATION_GRACE_PERIOD="5m" # shutdown grace period
KH_DEFAULT_CHECK_TIMEOUT="30s" # default check timeout
KH_DEFAULT_NAMESPACE="kuberhealthy" # fallback namespace
KH_TLS_CERT_FILE="" # TLS certificate path for HTTPS listener
KH_TLS_KEY_FILE="" # TLS private key path for HTTPS listener
KH_LEADER_ELECTION_ENABLED="false" # enable leader election for scheduling and reaping
KH_LEADER_ELECTION_NAME="kuberhealthy-controller" # lease name for leader election
KH_LEADER_ELECTION_NAMESPACE="kuberhealthy" # namespace to store the lease
KH_LEADER_ELECTION_LEASE_DURATION="15s" # lease duration for leader election
KH_LEADER_ELECTION_RENEW_DEADLINE="10s" # renew deadline for leader election
KH_LEADER_ELECTION_RETRY_PERIOD="2s" # retry period for leader election
```
