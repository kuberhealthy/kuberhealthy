# Viewing Kuberhealthy Check Status

Kuberhealthy exposes the state of every `khcheck` through two interfaces: the web status page and the `KuberhealthyCheck` custom resource.

## Status Page

The Kuberhealthy Service serves a read‑only JSON status page at `/status`.

### Example Output

```json
{
    "OK": true,
    "Errors": [],
    "CheckDetails": {
        "kuberhealthy/daemonset": {
            "OK": true,
            "Errors": [],
            "RunDuration": "22.512278967s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:24:16.7718171Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "9abd3ec0-b82f-44f0-b8a7-fa6709f759cd"
        },
        "kuberhealthy/deployment": {
            "OK": true,
            "Errors": [],
            "RunDuration": "29.142295647s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:26:40.7444659Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "5f0d2765-60c9-47e8-b2c9-8bc6e61727b2"
        },
        "kuberhealthy/dns-status-internal": {
            "OK": true,
            "Errors": [],
            "RunDuration": "2.43940936s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:34:04.8927434Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "c85f95cb-87e2-4ff5-b513-e02b3d25973a"
        },
        "kuberhealthy/pod-restarts": {
            "OK": true,
            "Errors": [],
            "RunDuration": "2.979083775s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:34:06.1938491Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "a718b969-421c-47a8-a379-106d234ad9d8"
        }
    },
    "CurrentMaster": "kuberhealthy-7cf79bdc86-m78qr"
}
```

The boolean `OK` field can be used to indicate global up/down status, while the `Errors` array contains all check error descriptions. Granular per-check information, including run duration, last run time, and the Kuberhealthy pod that executed the check, is available under the `CheckDetails` object.

### Using Port Forwarding

If the Service is only reachable inside the cluster, port‑forward to it:

```sh
kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:80
```

Then open `http://localhost:8080/status` in your browser.

### Through a Load Balancer or Ingress

When the Service is exposed externally, append `/status` to the load balancer or ingress URL. Authentication is handled by your provider or ingress controller. After logging in, the `/status` endpoint shows the health of every check.

## Inspecting `khcheck` Status Fields

Each `KuberhealthyCheck` resource records its last run information in the `status` block. View it with:

```sh
kubectl get khcheck <name> -o yaml
```

Key fields include:

- `ok` – whether the last run succeeded.
- `errors` – messages returned by the check.
- `consecutiveFailures` – number of sequential failed runs.
- `runDuration` – execution time of the last run.
- `namespace` – namespace where the check pod executed.
- `currentUUID` – identifier for the latest run.
- `lastRunUnix` – Unix time when the check last executed.
- `additionalMetadata` – optional metadata from the check.

These fields match the [`KuberhealthyCheckStatus`](../pkg/api/kuberhealthycheck_types.go) structure used by the operator.
