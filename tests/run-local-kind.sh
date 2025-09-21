#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="kuberhealthy"
IMAGE="docker.io/kuberhealthy/kuberhealthy:localdev"
TARGET_NAMESPACE="kuberhealthy"
KIND_VERSION="v1.29.0"
KH_CHECK_REPORT_URL="http://kuberhealthy.${TARGET_NAMESPACE}.svc.cluster.local:8080"
LOCAL_PORT=8080
# Ensure we use Podman for kind
export KIND_EXPERIMENTAL_PROVIDER=podman

# Track log follower PID so we can stop/restart when rebuilding
LOG_PID=""
PF_PID=""

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
    kind create cluster --name "$CLUSTER_NAME" --image "kindest/node:$KIND_VERSION"
  fi

  # Always export kubeconfig to ensure localhost endpoint and current context
  # are correctly set (especially important with the Podman provider on macOS).
  kind export kubeconfig --name "$CLUSTER_NAME"
}

build_and_load() {
  echo "📦 Building Podman image: $IMAGE"
  podman build -f cmd/kuberhealthy/Podfile -t "$IMAGE" .

  echo "📤 Loading image into kind"
  TMP_IMG_TAR="/tmp/kuberhealthy-image.tar"
  podman save "$IMAGE" -o "${TMP_IMG_TAR}"
  kind load image-archive "${TMP_IMG_TAR}" --name "$CLUSTER_NAME"
  rm -f "${TMP_IMG_TAR}"

  if kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" get deploy kuberhealthy >/dev/null 2>&1; then
    echo "🔁 Restarting deployment/kuberhealthy to pick up new image"
    kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" rollout restart deploy/kuberhealthy
  fi
}

start_logs() {
  echo "🪵 Tailing Kuberhealthy logs..."
  kubectl --context="kind-${CLUSTER_NAME}" get pod -n "$TARGET_NAMESPACE"
  set +e
  (
    # Ignore Ctrl-C inside this subshell so kubectl doesn't print a SIGINT error.
    trap '' INT
    exec kubectl --context="kind-${CLUSTER_NAME}" logs -n "$TARGET_NAMESPACE" -l app=kuberhealthy -f \
      2> >(sed '/Interrupted by SIG/d' >&2)
  ) &
  LOG_PID=$!
  set -e
}

stop_port_forward() {
  set +e
  if [[ -n "${PF_PID}" ]] && kill -0 "${PF_PID}" 2>/dev/null; then
    kill "${PF_PID}" 2>/dev/null || true
    wait "${PF_PID}" 2>/dev/null || true
  fi
  PF_PID=""
  set -e
}

start_port_forward() {
  stop_port_forward
  echo "🔌 Forwarding Kuberhealthy service to http://localhost:${LOCAL_PORT}"
  kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" port-forward service/kuberhealthy ${LOCAL_PORT}:8080 >/tmp/kuberhealthy-port-forward.log 2>&1 &
  PF_PID=$!
  sleep 1
  echo ""
  echo ""
  echo "🌐 View the dashboard at http://localhost:${LOCAL_PORT}"
  echo ""
  echo ""
  sleep 2
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
  stop_port_forward
  bash tests/cleanup-kind.sh || true
  echo "✅ KIND clean up complete."
  exit 0
}
trap cleanup INT

# Initial bring-up
ensure_cluster
build_and_load

echo "📤 Ensuring deployment manifest is applied"
kubectl --context="kind-${CLUSTER_NAME}" apply -k deploy/

echo "⏳ Waiting for Kuberhealthy deployment to apply..."
FOUND_DEPLOYMENT=false
for i in {1..30}; do
  if kubectl --context="kind-${CLUSTER_NAME}" get deployment kuberhealthy -n "$TARGET_NAMESPACE" &> /dev/null; then
    echo "✅ Kuberhealthy deployment exists"
    FOUND_DEPLOYMENT=true
    break
  else
    echo "⏱️ Waiting for deployment..."
    sleep 2
  fi
done
if [ "$FOUND_DEPLOYMENT" = false ]; then
  echo "‼️ Kuberhealthy deployment failed to deploy into KIND"
  exit 1
fi

echo "⏳ Waiting for Kuberhealthy rollout to complete..."
if ! kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" rollout status deploy/kuberhealthy --timeout=180s; then
  echo "‼️ Deployment rollout did not complete in time."
  kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" describe deploy/kuberhealthy || true
  exit 1
fi

LATEST_RS=$(kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" get rs -l app=kuberhealthy --sort-by=.metadata.creationTimestamp -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | tail -n1)
if [ -z "$LATEST_RS" ]; then
  echo "‼️ Unable to determine latest ReplicaSet for Kuberhealthy"
  kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" get rs -l app=kuberhealthy || true
  exit 1
fi

RS_HASH=$(kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" get rs "$LATEST_RS" -o jsonpath='{.metadata.labels.pod-template-hash}' 2>/dev/null || true)
if [ -z "$RS_HASH" ]; then
  RS_HASH="${LATEST_RS##*-}"
fi

echo "⏳ Waiting for pods from ReplicaSet ${LATEST_RS} to become ready..."
READY_POD=false
for i in {1..30}; do
  POD_COUNT=$(kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" get pods -l app=kuberhealthy,pod-template-hash="$RS_HASH" --no-headers 2>/dev/null | wc -l | tr -d ' ')
  if [ "${POD_COUNT:-0}" -gt 0 ]; then
    if kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" wait --for=condition=Ready pod -l app=kuberhealthy,pod-template-hash="$RS_HASH" --timeout=30s >/dev/null 2>&1; then
      echo "✅ Kuberhealthy pod from ${LATEST_RS} is ready"
      READY_POD=true
      break
    fi
  fi
  echo "⏱️ Waiting for new ReplicaSet pod..."
  sleep 2
done

if [ "$READY_POD" = false ]; then
  echo "‼️ Latest Kuberhealthy ReplicaSet pod did not become ready."
  kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" get pods -l app=kuberhealthy -o wide || true
  exit 1
fi

echo "⏳ Deploying test khcheck..."
kubectl --context="kind-${CLUSTER_NAME}" apply -n "$TARGET_NAMESPACE" -f tests/khcheck-test.yaml

start_logs
start_port_forward

echo ""
echo ""
echo "✨ Press enter to rebuild, redeploy, and reopen the dashboard."
echo "⛔️ Press Ctrl-C to tear down the KIND cluster and exit."
while true; do
  # Wait for an Enter keypress
  # shellcheck disable=SC2162
  read -r
  echo "🔁 Rebuilding image and re-deploying Kuberhealthy to KIND cluster..."
  stop_logs
  stop_port_forward
  ensure_cluster
  build_and_load
  echo "⏳ Waiting for deployment to complete..."
  kubectl --context="kind-${CLUSTER_NAME}" -n "$TARGET_NAMESPACE" rollout status deploy/kuberhealthy --timeout=120s || true
  start_logs
  start_port_forward
  echo "\n✅ Reload complete. Press Enter to rebuild again, or Ctrl-C to exit."
done
