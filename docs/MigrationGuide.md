# Migration Guide

Kuberhealthy v3 moves from the Helm chart on the `master` branch to a kustomize
application on the `main` branch and introduces a single
`KuberhealthyCheck` resource using the `kuberhealthy.github.io/v2` API. It
replaces the legacy `KuberhealthyCheck`, `KuberhealthyState`, and
`KuberhealthyJob` resources.

Benefits of switching:

- One simplified CRD – `KHCheck` now covers periodic checks and one‑time jobs
  via the `runOnce` flag.
- Mutating webhook keeps existing `comcast.github.io/v1` resources working so
  you can migrate at your own pace.
- Kustomize install from the `main` branch enables GitOps workflows and easier
  upgrades.
- Existing check clients remain compatible; only the operator changes.
- Re‑deploying the operator causes only a brief downtime for running checker
  pods.

## Step-by-step migration from Helm to kustomize

1. **Prepare** – Back up any Helm values and ensure `kubectl` has kustomize
   support. Re‑deploying the operator results in a small amount of downtime for
   running check pods.
2. **Uninstall the Helm chart** – Remove the old release:

   ```bash
   helm uninstall kuberhealthy -n kuberhealthy
   ```

   This deletes the deployment and old CRDs but leaves your `khcheck`
   resources.
3. **Install via kustomize** – Apply the manifests from the new branch:

   ```bash
   kubectl apply -k github.com/kuberhealthy/kuberhealthy/deploy?ref=main
   ```

   Wait for the `kuberhealthy` deployment to become ready.
4. **Verify existing checks** – Thanks to the mutating webhook, `khcheck`
   manifests that still specify `apiVersion: comcast.github.io/v1` continue to
   run without modification. Existing check clients also continue to work.
5. **Replace khjobs** – The legacy `khjob` resource is now a `KuberhealthyCheck`
   with `runOnce: true`. Apply the new manifest and delete the old `khjob`.
6. **Update branch references** – Any scripts or documentation referring to the
   `master` branch (for example raw manifest URLs) should now point to `main`.

After the migration you may delete the legacy CRDs
`khchecks.comcast.github.io`, `kuberhealthystates.comcast.github.io`, and
`khjobs.comcast.github.io` once no resources remain.

---

# KHCheck Migration Examples

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
  runOnce: true
  timeout: 15m
  podSpec:
    restartPolicy: Never
    containers:
    - name: backup
      image: example/backup-check:v3
status: {}
```

Apply any old `khcheck` manifests and Kuberhealthy will convert them to `KHCheck` automatically.
