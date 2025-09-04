#!/bin/bash
set -euo pipefail

CLUSTER_NAME="kuberhealthy-dev"
IMAGE="docker.io/kuberhealthy/kuberhealthy:localdev"
TARGET_NAMESPACE="kuberhealthy"
KH_CHECK_REPORT_HOSTNAME="kuberhealthy.${TARGET_NAMESPACE}.svc.cluster.local"
export KIND_EXPERIMENTAL_PROVIDER=podman

# Track log follower PID so we can stop/restart when rebuilding
LOG_PID=""

# Ensure kind is installed
if ! command -v kind &>/dev/null; then
  echo "❌ kind not found in PATH. Aborting."
  exit 1
fi
echo "kind found in PATH"

# Ensure kustomize is installed
if ! command -v kustomize &>/dev/null; then
  echo "❌ kustomize not found in PATH. Aborting."
  exit 1
fi
echo "kustomize found in PATH"

ensure_cluster() {
  if kind get clusters | grep -q "$CLUSTER_NAME"; then
    echo "✅ Reusing existing kind cluster: $CLUSTER_NAME"
  else
    echo "🚀 Creating kind cluster: $CLUSTER_NAME"
    kind create cluster --name "$CLUSTER_NAME" --image kindest/node:v1.29.0
  fi
}

build_and_load() {
  echo "📦 Building Podman image: $IMAGE"
  podman build -f cmd/kuberhealthy/Podfile -t "$IMAGE" .

  echo "📤 Loading image into kind"
  TMP_IMG_TAR="/tmp/kuberhealthy-image.tar"
  podman save "$IMAGE" -o "${TMP_IMG_TAR}"
  kind load image-archive "${TMP_IMG_TAR}" --name "$CLUSTER_NAME"
  rm -f "${TMP_IMG_TAR}"

  if kubectl -n "$TARGET_NAMESPACE" get deploy kuberhealthy >/dev/null 2>&1; then
    echo "🔁 Restarting deployment/kuberhealthy to pick up new image"
    kubectl -n "$TARGET_NAMESPACE" rollout restart deploy/kuberhealthy
  fi
}

start_logs() {
  echo "🪵 Tailing Kuberhealthy logs..."
  kubectl get pod -n "$TARGET_NAMESPACE"
  set +e
  kubectl logs -n "$TARGET_NAMESPACE" -l app=kuberhealthy -f &
  LOG_PID=$!
  set -e
}

stop_logs() {
  set +e
  if [[ -n "${LOG_PID}" ]] && kill -0 "${LOG_PID}" 2>/dev/null; then
    kill "${LOG_PID}" 2>/dev/null || true
    wait "${LOG_PID}" 2>/dev/null || true
  fi
  set -e
}

cleanup() {
  echo "\n🧹 Cleaning up..."
  stop_logs
  bash tests/cleanup-kind.sh || true
  echo "✅ Teardown complete. Bye!"
}
trap cleanup INT

# Initial bring-up
ensure_cluster
build_and_load

echo "📤 Ensuring manifests are applied"
kustomize build deploy/ | kubectl apply -f -

echo "⏳ Waiting for Kuberhealthy deployment to apply..."
FOUND_DEPLOYMENT=FALSE
for i in {1..30}; do
  if kubectl get deployment kuberhealthy -n kuberhealthy &> /dev/null; then
    echo "✅ Kuberhealthy deployment exists"
    FOUND_DEPLOYMENT=TRUE
    break
  else
    echo "⏱️ Waiting for deployment..."
    sleep 2
  fi
done
if [ "$FOUND_DEPLOYMENT" = false ]; then
  echo "‼️ Kuberhealthy deployment did not appear in KIND cluster"
  exit 1
fi

echo "⏳ Waiting for Kuberhealthy pods to be online..."
FOUND_POD=FALSE
for i in {1..30}; do
  if kubectl get pods -n kuberhealthy -l app=kuberhealthy --no-headers 2>/dev/null | grep -v Pending | grep -v ContainerCreating | grep -q .; then
    echo "✅ Kuberhealthy pod exists"
    FOUND_POD=TRUE
    break
  else
    echo "⏱️ Waiting for pod..."
    sleep 2
  fi
done

if [ "$FOUND_POD" = false ]; then
  echo "‼️ Pod did not appear, running log command for troubleshooting..."
  kubectl logs -n "$TARGET_NAMESPACE" -l app=kuberhealthy
  exit 1
fi

start_logs

echo "\n✨ Press Enter to rebuild and re-ship the image."
echo "   Press Ctrl-C to tear down the kind cluster and exit."
while true; do
  # Wait for an Enter keypress
  # shellcheck disable=SC2162
  read -r
  echo "🔁 Rebuilding image and re-shipping to kind..."
  stop_logs
  ensure_cluster
  build_and_load
  echo "⏳ Waiting for rollout to complete..."
  kubectl -n "$TARGET_NAMESPACE" rollout status deploy/kuberhealthy --timeout=120s || true
  start_logs
  echo "\n✅ Reload complete. Press Enter to rebuild again, or Ctrl-C to exit."

done

# Should not be reached; Ctrl-C trap handles teardown
