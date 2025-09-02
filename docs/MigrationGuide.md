# KHCheck Migration Examples

Kuberhealthy v3 introduces a single `KuberhealthyCheck` resource using the `kuberhealthy.github.io/v2` API. It replaces the legacy `KuberhealthyCheck`, `KuberhealthyState`, and `KuberhealthyJob` resources.

Existing manifests continue to work. A mutating webhook automatically converts old resources on apply, making migration straightforward.

## khcheck and khstate

Legacy resources:

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: pod-restarts
spec:
  runInterval: 1m
  podSpec:
    containers:
    - name: pod-restarts
      image: kuberhealthy/pod-restart-check:v2
---
apiVersion: comcast.github.io/v1
kind: KuberhealthyState
metadata:
  name: pod-restarts
```

New `KHCheck`:

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: KuberhealthyCheck
metadata:
  name: pod-restarts
spec:
  runInterval: 1m
  podSpec:
    containers:
    - name: pod-restarts
      image: ghcr.io/kuberhealthy/pod-restart-check:v3
status: {}
```

## khjob

Legacy resource:

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyJob
metadata:
  name: backup
spec:
  jobSpec:
    template:
      spec:
        restartPolicy: Never
        containers:
        - name: backup
          image: example/backup-check:v1
```

New `KHCheck`:

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: KuberhealthyCheck
metadata:
  name: backup
spec:
  runInterval: 0
  timeout: 15m
  podSpec:
    restartPolicy: Never
    containers:
    - name: backup
      image: example/backup-check:v3
status: {}
```

Apply any old `khcheck` manifests and Kuberhealthy will convert them to `KHCheck` automatically.
