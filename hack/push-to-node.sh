#!/usr/bin/env bash
set -euo pipefail
[ "${DEBUG:-}" = "1" ] && set -x

# One-stop: sync build context to node, build image on that node (matching its arch),
# ensure image is available in containerd, apply/update manifests, and list pods.

IMAGE="kuberhealthy:localdev"
NODE="${NODE:-k8s1}"
# Prefer $USER when available to avoid numeric whoami on some systems
REMOTE_USER="${REMOTE_USER:-${USER:-$(whoami)}}"
REMOTE="${REMOTE_USER}@${NODE}"
REMOTE_BUILD_DIR="${REMOTE_BUILD_DIR:-/tmp/kuberhealthy-build}"
NAMESPACE="${NAMESPACE:-kuberhealthy}"

bold() { printf "\033[1m%s\033[0m\n" "$*"; }

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing dependency: $1" >&2
    exit 1
  fi
}

bold "Checking local prerequisites"
need kubectl
need ssh
if ! command -v rsync >/dev/null 2>&1; then
  echo "rsync not found locally; will fallback to tar+ssh for context sync"
fi

if ! command -v kustomize >/dev/null 2>&1; then
  echo "kustomize not found; attempting to use 'kubectl kustomize'"
  if ! kubectl kustomize >/dev/null 2>&1; then
    echo "Neither kustomize nor 'kubectl kustomize' is available." >&2
    exit 1
  fi
fi

bold "Syncing source to ${REMOTE}:${REMOTE_BUILD_DIR}"
if [ "${FORCE_TAR_SYNC:-}" = "1" ]; then
  RSYNC_OK=1 # skip rsync
else
  RSYNC_OK=0
  if command -v rsync >/dev/null 2>&1; then
    # Try rsync first; if it fails, fall back to tar+ssh
    if rsync -az --delete --exclude .git --exclude vendor ./ "$REMOTE:$REMOTE_BUILD_DIR/"; then
      RSYNC_OK=1
    fi
  fi
fi

if [ "$RSYNC_OK" -ne 1 ]; then
  TMP_TAR="kuberhealthy-src.tar.gz"
  tar -czf "$TMP_TAR" --exclude .git --exclude vendor .
  scp -q "$TMP_TAR" "$REMOTE:/tmp/$TMP_TAR"
  ssh "$REMOTE" "mkdir -p '$REMOTE_BUILD_DIR' && tar -xzf /tmp/$TMP_TAR -C '$REMOTE_BUILD_DIR' && rm -f /tmp/$TMP_TAR"
  rm -f "$TMP_TAR"
fi

bold "Preparing remote root build script"
REMOTE_SCRIPT="$REMOTE_BUILD_DIR/hack/build_as_root.sh"
ssh "$REMOTE" "mkdir -p '$REMOTE_BUILD_DIR/hack'"
ssh "$REMOTE" 'cat > '"$REMOTE_SCRIPT"' <<\'REMOTE_SCRIPT_EOF'
#!/usr/bin/env bash
set -euo pipefail
[ "${DEBUG:-}" = "1" ] && set -x
IMAGE="${IMAGE:-docker.io/kuberhealthy/kuberhealthy:localdev}"
BUILD_DIR="${BUILD_DIR:-/tmp/kuberhealthy-build}"
PODFILE="${PODFILE:-$BUILD_DIR/cmd/kuberhealthy/Podfile}"

echo "IMAGE: $IMAGE"
echo "BUILD_DIR: $BUILD_DIR"
echo "PODFILE: $PODFILE"

CTR_BIN="$(command -v ctr || true)"
if [ -z "$CTR_BIN" ]; then
  for p in /usr/bin/ctr /usr/local/bin/ctr; do
    if [ -x "$p" ]; then CTR_BIN="$p"; break; fi
  done
fi
if [ -z "$CTR_BIN" ]; then
  echo "ctr not found on remote node" >&2
  exit 1
fi

success=0

if command -v nerdctl >/dev/null 2>&1; then
  echo "Trying nerdctl to build directly into containerd (k8s.io)"
  if nerdctl --namespace k8s.io build -t "$IMAGE" -f "$PODFILE" "$BUILD_DIR"; then
    success=1
  else
    echo "nerdctl build failed; will try docker/podman" >&2
  fi
fi

if [ "$success" -eq 0 ] && command -v podman >/dev/null 2>&1; then
  echo "Trying podman to build, then import into containerd"
  if podman build -t "$IMAGE" -f "$PODFILE" "$BUILD_DIR" && \
     podman image save --format oci-archive "$IMAGE" | "$CTR_BIN" -n k8s.io images import -; then
    success=1
  else
    echo "podman path failed; will try docker if present" >&2
  fi
fi

if [ "$success" -eq 0 ] && command -v docker >/dev/null 2>&1; then
  echo "Trying docker to build, then import into containerd"
  if docker build -t "$IMAGE" -f "$PODFILE" "$BUILD_DIR" && \
     docker save "$IMAGE" | "$CTR_BIN" -n k8s.io images import -; then
    success=1
  else
    echo "docker path failed" >&2
  fi
fi

if [ "$success" -ne 1 ]; then
  echo "No working container runtime succeeded on remote node (tried nerdctl, docker, podman)." >&2
  exit 1
fi
REMOTE_SCRIPT_EOF'

bold "Running remote root build script"
ssh -tt "$REMOTE" sudo env DEBUG="${DEBUG:-}" IMAGE="$IMAGE" BUILD_DIR="$REMOTE_BUILD_DIR" PODFILE="$REMOTE_BUILD_DIR/cmd/kuberhealthy/Podfile" bash "$REMOTE_SCRIPT"

bold "Ensuring namespace '${NAMESPACE}' exists"
kubectl get ns "$NAMESPACE" >/dev/null 2>&1 || kubectl create ns "$NAMESPACE"

bold "Applying Kuberhealthy manifests via kustomize (pinned to node k8s1)"
if command -v kustomize >/dev/null 2>&1; then
  kustomize build deploy/overlays/k8s1 | kubectl apply -f -
else
  kubectl kustomize deploy/overlays/k8s1 | kubectl apply -f -
fi

bold "Restarting deployment to pick up updated local image"
kubectl -n "$NAMESPACE" rollout restart deployment/kuberhealthy || true

bold "Waiting for rollout"
kubectl -n "$NAMESPACE" rollout status deployment/kuberhealthy --timeout=300s

bold "Listing pods"
kubectl -n "$NAMESPACE" get pods -o wide

bold "Done"
