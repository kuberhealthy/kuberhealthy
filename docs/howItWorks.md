# How Kuberhealthy Works

Kuberhealthy watches the Kubernetes API for `KuberhealthyCheck` resources and continuously records their success or failure. It empowers you to run synthetic checks that simulate real user behavior so issues surface exactly as your users would experience them.

## 📚 Table of Contents

- [🚀 From Check Creation to Results](#from-check-creation-to-results)
- [🧾 Using the JSON Status Page](#using-the-json-status-page)
- [🕒 Run Once Checks](runOnceChecks.md)

## From Check Creation to Results

Imagine you want to verify that Kubernetes deployments function correctly. You start by applying a manifest for the built-in deployment check:

```sh
kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/master/cmd/deployment-check/deployment-check.yaml
```

As soon as the manifest is applied, Kuberhealthy orchestrates the following cycle:

1. **Detect the check.** Kuberhealthy sees the new `KuberhealthyCheck` resource.
2. **Schedule runs.** It begins creating checker pods on the interval specified in the resource.
3. **Start the pod.** Kubernetes launches the pod just like any other workload.
4. **Run the logic.** The container executes your test logic and decides whether the cluster behaved correctly.
5. **Enforce deadlines.** If the pod runs longer than the configured timeout, Kuberhealthy stops it and records a failure.
6. **Report in.** The pod calls back to the Kuberhealthy API at the `KH_REPORTING_URL`, sharing a boolean OK value and any error messages.
7. **Persist status.** Kuberhealthy writes the reported state to the `status` block of the `khcheck` so you can read it later.
8. **Publish metrics.** The stored OK and error values become Prometheus metrics served from the `/metrics` endpoint.

Behind the scenes, the deployment check follows the same create, observe, and clean-up loop you would execute manually:

- Kuberhealthy observes the new [`KuberhealthyCheck`](CHECKS.md#khcheck-anatomy).
- The controller schedules a checker pod according to the configured interval.
- That pod creates a deployment with the Kubernetes API and waits for every replica to become `Ready`.
- Once the verification succeeds, the pod deletes the deployment and waits for the cleanup to finish.
- The pod reports a success (or failure) back to the Kuberhealthy API, which stores the result and publishes metrics.

<img src="../assets/kh-ds-check.gif" alt="Kuberhealthy deployment check illustration" />

You can follow along with the run lifecycle directly from the cluster:

```sh
kubectl -n kuberhealthy get khcheck deployment -o wide
kubectl -n kuberhealthy describe khcheck deployment
```

The wide view shows the current phase and last run time, while the describe output expands on the stored `status` block.

Once a check has reported in, explore the results in three ways:

1. **Check the resource:** `kubectl -n kuberhealthy describe khcheck deployment` displays the `status.ok` flag and any reported errors.
2. **Inspect the status page:** port-forward the service with `kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:80` and then visit `http://localhost:8080/status` in your browser or call it with `curl -fsS localhost:8080/status`.
3. **View the metrics:** with the same port-forward session, run `curl -fsS localhost:8080/metrics | grep kuberhealthy_check` to see the Prometheus series that includes the deployment check.

## Using the JSON Status Page

Kuberhealthy exposes the state of every `khcheck` through two interfaces: the web status page and the `KuberhealthyCheck` custom resource. The service hosts a read-only JSON document at `/status` that aggregates every registered check:

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
        }
    },
    "CurrentMaster": "kuberhealthy-7cf79bdc86-m78qr"
}
```

Use `kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:80` if the service is only reachable inside the cluster. The document shows global health via the top-level `OK` flag, while the `CheckDetails` section lists run-level diagnostics for every check.

Each `khcheck` resource records the same information in its `status` block. View it with:

```sh
kubectl -n kuberhealthy describe khcheck <name>
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
