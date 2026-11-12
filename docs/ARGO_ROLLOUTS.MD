# Argo Rollouts and Kargo Analysis

Kuberhealthy results can gate a rollout. An Argo Rollouts `AnalysisTemplate` can read them from the `/json` status endpoint or from the Prometheus metrics, and fail the analysis when a HealthCheck fails. Kargo uses the same `AnalysisTemplate` resources for Stage verification, so the templates below work there too.

The analysis reads the most recent result of each HealthCheck. It doesn't start a check run, so pick an `interval` and `count` that cover at least one `runInterval` of the checks you care about.

## Using the JSON status endpoint

`GET /json` returns a top-level `OK` that is `true` only when every HealthCheck passed its last run. Use `?namespace=` to only look at the checks in some namespaces. See [HTTP API](HTTP_API.MD).

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AnalysisTemplate
metadata:
  name: kuberhealthy
spec:
  args:
    - name: namespace
  metrics:
    - name: kuberhealthy-checks
      interval: 1m
      count: 5
      failureLimit: 1
      successCondition: result == true
      provider:
        web:
          url: "http://kuberhealthy.kuberhealthy.svc.cluster.local/json?namespace={{args.namespace}}"
          jsonPath: "{$.OK}"
```

Reference it from a Rollout step:

```yaml
strategy:
  canary:
    steps:
      - setWeight: 20
      - analysis:
          templates:
            - templateName: kuberhealthy
          args:
            - name: namespace
              value: my-app
```

Each entry in `CheckDetails` also has its own `ok` field, keyed by HealthCheck name, if you want a `jsonPath` that only looks at one check.

## Using Prometheus metrics

If Prometheus scrapes Kuberhealthy, the `kuberhealthy_check` gauge is `1` for a passing check and `0` for a failing one. See [Metrics Catalog](METRICS_CATALOG.MD).

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AnalysisTemplate
metadata:
  name: kuberhealthy-check
spec:
  args:
    - name: namespace
    - name: check
  metrics:
    - name: kuberhealthy-check
      interval: 1m
      count: 5
      failureLimit: 1
      successCondition: result[0] == 1
      provider:
        prometheus:
          address: http://prometheus.monitoring.svc.cluster.local:9090
          query: |
            min(kuberhealthy_check{namespace="{{args.namespace}}", check="{{args.check}}"}) or vector(0)
```

The `or vector(0)` makes the analysis fail when the check has no series yet, for example when the name is wrong, instead of returning no data.

## Kargo

Point a Stage's verification at the template:

```yaml
spec:
  verification:
    analysisTemplates:
      - name: kuberhealthy
    args:
      - name: namespace
        value: my-app
```

The `AnalysisTemplate` has to be in the Stage's project namespace.
