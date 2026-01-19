# Quickstart

This guide installs Kuberhealthy, deploys a sample `HealthCheck`, and shows how to verify results.

## Install

Kustomize:

```sh
kubectl apply -k github.com/kuberhealthy/kuberhealthy/deploy/kustomize/base?ref=main
```
Tracks the latest `main` commit. Update to a release tag once tags are published.

Helm:

```sh
helm install kuberhealthy deploy/helm/kuberhealthy -n kuberhealthy --create-namespace
```
Pin to a release tag when chart packages are available.

## Apply a sample HealthCheck

```sh
kubectl apply -f - <<'YAML'
apiVersion: kuberhealthy.github.io/v2
kind: HealthCheck
metadata:
  name: quickstart-check
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 1m
  podSpec:
    spec:
      containers:
        - name: check
          image: docker.io/curlimages/curl:8.5.0
          imagePullPolicy: IfNotPresent
          command:
            - sh
            - -c
            - |
              set -euo pipefail
              echo "Reporting to $KH_REPORTING_URL"
              curl -fsS \
                -H "Content-Type: application/json" \
                -H "kh-run-uuid: $KH_RUN_UUID" \
                -d '{"ok":true,"errors":[]}' "$KH_REPORTING_URL"
      restartPolicy: Never
YAML
```

## Verify

```sh
kubectl -n kuberhealthy get healthchecks
kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:80
curl -fsS http://localhost:8080/json
```

Open `http://localhost:8080` to view the status UI.

## Uninstall

Kustomize:

```sh
kubectl delete -k deploy/kustomize/base
```

Helm:

```sh
helm uninstall kuberhealthy -n kuberhealthy
```

Optional:

```sh
kubectl delete namespace kuberhealthy
```
