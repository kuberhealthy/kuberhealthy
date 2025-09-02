# Viewing Kuberhealthy Check Status

Kuberhealthy exposes the state of every `khcheck` through two interfaces: the web status page and the `KuberhealthyCheck` custom resource.

## Status Page

The Kuberhealthy Service serves a read‑only JSON status page at `/status`.

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
