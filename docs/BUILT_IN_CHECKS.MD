# Built-in HTTP and TCP Checks

A lot of checks only need to answer "can I reach this and does it look right". For those you don't need a checker image. A `HealthCheck` can describe an HTTP request or a TCP connection, and Kuberhealthy runs it itself on every interval.

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: HealthCheck
metadata:
  name: payments-api
  namespace: kuberhealthy
spec:
  runInterval: 1m
  timeout: 30s
  http:
    url: https://payments.payments.svc:8443/healthz
```

Set only one of `podSpec`, `http` or `tcp`. The API server rejects a `HealthCheck` that sets more than one.

## How a built-in check runs

On each run Kuberhealthy does the same bookkeeping as for a pod check: it stores a new run UUID and the run start time, and it starts the timeout watcher. Then, instead of creating a pod, it sends the request or opens the connection from its own pod, bounded by `spec.timeout`.

The result is written to the same `status` fields a checker pod report sets, so these all work the same as for pod checks:

- the web UI and `/json`
- the `kuberhealthy_check` metrics and the consecutive failure count
- the `CheckReported` events on the `HealthCheck`
- Argo Rollouts and Kargo gates (see [ARGO_ROLLOUTS.MD](ARGO_ROLLOUTS.MD))

A run that takes longer than `spec.timeout` fails, like a pod that never reported. A result that arrives after the run timed out, or after a newer run started, is dropped.

Because no pod is scheduled, a built-in check starts in milliseconds, has no image to pull or keep patched, and leaves nothing for the reaper to clean up. It also needs no service account or RBAC of its own.

## Where the probe comes from

The request is sent from the Kuberhealthy pod, not from a pod in the target namespace. That matters in a few places:

- NetworkPolicies on the target have to allow ingress from the Kuberhealthy pod. If you want to test reachability from a specific namespace or node, use a pod check instead.
- DNS is resolved with the Kuberhealthy pod's resolver, so short names like `payments` resolve in the Kuberhealthy namespace. Use `payments.payments.svc` or the full name.
- `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` set on the Kuberhealthy deployment apply to HTTP checks. Add cluster-internal domains to `NO_PROXY` if you set a proxy.

## HTTP checks

```yaml
spec:
  runInterval: 1m
  timeout: 30s
  http:
    url: https://payments.payments.svc:8443/healthz
    method: GET
    expectedStatusCodes: [200, 204]
    bodyContains: '"status":"ok"'
    headers:
      X-Probe: kuberhealthy
```

| Field | Default | Description |
| --- | --- | --- |
| `url` | required | `http://` or `https://` address to request. |
| `method` | `GET` | HTTP method. The request has no body. |
| `expectedStatusCodes` | any 2xx | Status codes that count as success. |
| `bodyContains` | none | A string the response body must contain. |
| `bodyRegex` | none | A [Go regular expression](https://pkg.go.dev/regexp/syntax) the body must match. |
| `headers` | none | Headers added to the request. |
| `insecureSkipVerify` | `false` | Skip TLS certificate verification. |

Redirects are followed, and the status code of the final response is checked. When `bodyContains` or `bodyRegex` is set, only the first 1 MiB of the body is read, so put the marker near the start of large responses. If both are set, both have to match.

The check fails with an error that says what went wrong, for example:

```text
GET https://payments.payments.svc:8443/healthz returned 503, expected 200 or 204
body did not contain "\"status\":\"ok\""
```

### Checking a status page for a field

`bodyRegex` is handy when the value can vary but the shape can't:

```yaml
  http:
    url: http://search.search.svc:9200/_cluster/health
    bodyRegex: '"status":"(green|yellow)"'
```

### Checking an external dependency

```yaml
  http:
    url: https://api.example.com/v1/ping
    expectedStatusCodes: [200]
```

This tests egress from the cluster too, so it fails if the cluster loses outbound access or DNS for external names.

### Headers and secrets

Header values are stored in the `HealthCheck` in plain text, and anyone who can read `HealthCheck` resources can see them. Don't put tokens or passwords there. For an endpoint that needs credentials, use a pod check that reads them from a Secret.

## TCP checks

```yaml
spec:
  runInterval: 30s
  timeout: 10s
  tcp:
    address: postgres.db.svc:5432
```

| Field | Default | Description |
| --- | --- | --- |
| `address` | required | `host:port` to connect to. IPv6 addresses go in brackets, e.g. `[fd00::1]:5432`. |

The check passes when the connection opens, and Kuberhealthy closes it right away. It doesn't send any data or do a TLS handshake, so it tells you the port is reachable, not that the service behind it works.

## When to use a pod check instead

Built-in checks cover reachability and simple content checks. Write a pod check (see [CHECK_CREATION.md](CHECK_CREATION.md)) when you need to:

- probe from a particular namespace, node or service account
- send credentials from a Secret, or a request body
- run several steps, like logging in and then calling an API
- check something other than HTTP or TCP, such as DNS records or certificate expiry

## Examples

Ready-to-apply examples are in [healthchecks/HTTP_BUILT_IN_CHECK.yaml](healthchecks/HTTP_BUILT_IN_CHECK.yaml) and [healthchecks/TCP_BUILT_IN_CHECK.yaml](healthchecks/TCP_BUILT_IN_CHECK.yaml). The full field reference is in [CRD_REFERENCE.MD](CRD_REFERENCE.MD).
