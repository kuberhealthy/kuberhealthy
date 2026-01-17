# How Kuberhealthy Works

Kuberhealthy watches `HealthCheck` resources, schedules checker pods, and records results. Those results appear in the `status` block and as Prometheus metrics.

## Flow

1. You apply a `HealthCheck` resource.
2. The controller schedules checker pods on the configured interval.
3. The pod runs your test logic.
4. The pod reports `OK`/`Errors` to `KH_REPORTING_URL` (the full `/check` endpoint injected into check pods).
5. Kuberhealthy stores the result and exports metrics on `/metrics`.

Use a built-in example to see the lifecycle:

```sh
kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/deployment-check/main/healthcheck.yaml
```

## View status

- Resource status:
  ```sh
  kubectl -n kuberhealthy describe healthcheck deployment
  ```
- JSON status page:
  ```sh
  kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:80
  curl -fsS localhost:8080/json
  ```
- Metrics:
  ```sh
  curl -fsS localhost:8080/metrics | grep kuberhealthy_check
  ```

The JSON document aggregates every check and mirrors the same fields stored in `status`.

## Run once checks

See [RUNONCECHECKS.MD](RUNONCECHECKS.MD) for one-shot validation runs.
