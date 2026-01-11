# Check Migration Playbook (V2 -> V3)

This document captures the full process used to migrate the v2 `deployment-check` into a standalone v3-compatible repository. It is intended for another AI agent to repeat this for a different check, end-to-end, with concrete steps, gotchas, and fixes.

## High-Level Goal

- Extract a v2 check from `kuberhealthy/kuberhealthy` `master` into a new repo under `kuberhealthy/<check-name>`.
- Rewrite the check to work with Kuberhealthy v3 `HealthCheck` CRD (no `KuberhealthyCheck`, `KHCheck`, `KHState`, `KHJob`).
- Ensure the repo has:
  - `Containerfile`
  - `Justfile`
  - `README.md`
  - `healthcheck.yaml` example
  - GitHub Actions to publish images to Docker Hub
- Validate the check in a Kubernetes cluster with Kuberhealthy v3 installed.

## Repos and Links Used (Example)

- Source v2 repo: `https://github.com/kuberhealthy/kuberhealthy` (branch `master`)
- Target v3 repo: `https://github.com/kuberhealthy/deployment-check`
- V3 `HealthCheck` schema: `deploy/helm/kuberhealthy3/crds/healthchecks.kuberhealthy.github.io.yaml`
- Kuberhealthy v3 docs: `docs/CHECK_CREATION.md`

## Step-by-Step Process

### 1) Inspect v2 check in `master`

Example for deployment check:

- `cmd/deployment-check/main.go`
- `cmd/deployment-check/input.go`
- `cmd/deployment-check/run_check.go`
- `cmd/deployment-check/deployment.go`
- `cmd/deployment-check/service.go`
- `cmd/deployment-check/service_requester.go`
- `cmd/deployment-check/deployment-check.yaml`
- `cmd/deployment-check/README.md`

Key things to extract:

- Environment variables (defaults and names)
- Control flow (create, wait, request, cleanup)
- Any rolling update logic
- RBAC rules (deployments, services, pods, events)

### 2) Identify v3 compatibility requirements

From `main` (v3):

- Reporting uses `github.com/kuberhealthy/kuberhealthy/v3/pkg/checkclient`.
- Deadline env: `KH_CHECK_RUN_DEADLINE`.
- Reporting url env: `KH_REPORTING_URL`.
- UUID env: `KH_RUN_UUID`.
- `HealthCheck` resource schema:
  - `apiVersion: kuberhealthy.github.io/v2`
  - `kind: HealthCheck`
  - `spec.podSpec.spec` (note extra `spec` nesting compared to v2 KuberhealthyCheck).

### 3) Create the new repo

- Create repo in org, public.
- If you cannot create via API, ask a user to create it. Use the exact `kuberhealthy/<check-name>` path.

Example repo URL (deployment check):

```
https://github.com/kuberhealthy/deployment-check
```

### 4) Scaffold the Go repo

For a single binary repo:

```
.
├── cmd
│   └── deployment-check
├── Containerfile
├── Justfile
├── README.md
├── healthcheck.yaml
├── go.mod
└── go.sum
```

### 5) Rewrite the check to v3

Use v2 logic, but replace v2 packages and requirements.

#### Required package rewrites

- `github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient` ->
  `github.com/kuberhealthy/kuberhealthy/v3/pkg/checkclient`
- `github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/nodeCheck` ->
  `github.com/kuberhealthy/kuberhealthy/v3/pkg/nodecheck`
- `github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient` -> custom client helper:
  - Try `rest.InClusterConfig()`, fallback to `~/.kube/config`.
  - Use `kubernetes.NewForConfig` typed clientset.

#### Deadline handling (v3)

```go
// Uses KH_CHECK_RUN_DEADLINE.
checkTimeLimit := defaultCheckTimeLimit
deadline, err := checkclient.GetDeadline()
if err == nil {
	checkTimeLimit = deadline.Sub(time.Now().Add(time.Second * 5))
}
```

#### Reporting (v3)

```go
err := checkclient.ReportSuccess()
err := checkclient.ReportFailure([]string{"error message"})
```

### 6) Maintain behavior parity

Keep the old semantics intact:

- same env vars
- same default values
- same resource naming
- same rollout behavior
- same HTTP backoff (including Istio 502 workaround)

### 7) Known issues and fixes

#### Probe handler required

Kubernetes requires probes to have a handler type. v2 check relied on default behavior and never set `TCPSocket`, `HTTPGet`, etc. In v3 rewrite you must add one.

Example fix:

```go
liveProbe := corev1.Probe{...}
liveProbe.TCPSocket = &corev1.TCPSocketAction{
	Port: intstr.FromInt(int(containerPort)),
}
```

#### Deployment update requires resourceVersion

v2 update logic used a new deployment object without `resourceVersion`, which fails with `Update()`.

Fix:

- Fetch the existing deployment
- Update the spec template and image
- Call Update with the updated object

#### Foreground deletion stalls

Foreground deletion can hang when pods are stuck with finalizers. Switch to background delete:

```go
deletePolicy := metav1.DeletePropagationBackground
```

### 8) GitHub Actions for images

Requirement: only tag images on semver tag, default pushes tag image with short SHA.

Example `publish.yaml` (final):

```yaml
name: Publish

on:
  push:
    branches:
      - main
    tags:
      - "v*"

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKER_HUB_USER }}
          password: ${{ secrets.DOCKER_HUB_PAT }}

      - name: Docker metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: kuberhealthy/<check-name>
          tags: |
            type=sha,format=short
            type=ref,event=tag

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          file: ./Containerfile
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
```

### 9) Containerfile and Justfile

#### `Containerfile`

```Dockerfile
FROM golang:1.24 AS builder
WORKDIR /build
COPY go.mod /build/
RUN go mod download
COPY . /build
ENV CGO_ENABLED=0
RUN go build -v -o /build/bin/<check-name> ./cmd/<check-name>
RUN groupadd -g 999 user && useradd -r -u 999 -g user user

FROM scratch
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/bin/<check-name> /app/<check-name>
USER user
ENTRYPOINT ["/app/<check-name>"]
```

#### `Justfile`

```make
IMAGE := "kuberhealthy/<check-name>"
TAG := "latest"

build:
	podman build -f Containerfile -t {{IMAGE}}:{{TAG}} .

test:
	go test ./...

binary:
	go build -o bin/<check-name> ./cmd/<check-name>
```

### 10) HealthCheck example

Example resource (v3):

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: HealthCheck
metadata:
  name: <check-name>
  namespace: kuberhealthy
spec:
  runInterval: 10m
  timeout: 15m
  podSpec:
    spec:
      containers:
        - name: <check-name>
          image: kuberhealthy/<check-name>:latest
          env:
            - name: CHECK_SOME_VALUE
              value: "true"
      restartPolicy: Never
```

### 11) RBAC bundle

Include a `healthcheck.yaml` with all required RBAC.

Example pattern:

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: <check-name>-role
  namespace: kuberhealthy
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["create","delete","get","list","patch","update","watch"]
  - apiGroups: [""]
    resources: ["services","pods","events"]
    verbs: ["create","delete","get","list","patch","update","watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: <check-name>-rb
  namespace: kuberhealthy
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: <check-name>-role
subjects:
  - kind: ServiceAccount
    name: <check-name>-sa
    namespace: kuberhealthy
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: <check-name>-sa
  namespace: kuberhealthy
```

### 12) Verification steps

#### Local tests

```bash
just test
```

#### GitHub Actions verification

- Confirm a workflow run triggered by `main` push.
- Confirm Docker Hub updated with the short SHA tag.

#### Cluster validation

1. Install Kuberhealthy v3 in cluster (example uses ArgoCD and helm chart path):

```
repo: https://github.com/kuberhealthy/kuberhealthy
path: deploy/helm/kuberhealthy3
revision: main
```

2. Apply `healthcheck.yaml`.

3. Confirm check pod succeeds and logs show:
   - deployment created
   - service reachable
   - rolling update (if enabled)
   - cleanup successful
   - report success

### 13) Real issues encountered and fixes used

- **Probe handlers missing**: added TCP socket probes for liveness/readiness.
- **Deployment update failing**: fetch current deployment before update to preserve `resourceVersion`.
- **Cleanup stuck on foreground deletion**: switched to background propagation.
- **Old deployment lingering**: added pre-run cleanup to remove old deployments and services.

### 14) Final confirmation checklist

- Repo structure matches `cmd/<check-name>`
- `Containerfile` builds correct path
- `Justfile` builds correct path
- `publish.yaml` tags SHA on branch push, semver on tag push
- `README.md` includes config and example `HealthCheck`
- `healthcheck.yaml` includes RBAC
- Check pod runs and reports success in cluster

