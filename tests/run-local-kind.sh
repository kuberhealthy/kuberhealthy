#!/bin/bash
set -euo pipefail

CLUSTER_NAME="kuberhealthy-dev"
IMAGE="docker.io/kuberhealthy/kuberhealthy:localdev"
TARGET_NAMESPACE="kuberhealthy"
KH_CHECK_REPORT_URL="http://kuberhealthy.${TARGET_NAMESPACE}.svc.cluster.local/externalCheckStatus"

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

echo "📦 Building Docker image: $IMAGE"
# this is meant to be run with `just kind` from the root of the repo
docker build -f cmd/kuberhealthy/Containerfile -t "$IMAGE" .

# Delete existing cluster
if kind get clusters | grep -q "$CLUSTER_NAME"; then
  echo "🧹 Deleting existing kind cluster: $CLUSTER_NAME"
  kind delete cluster --name "$CLUSTER_NAME"
fi

echo "🚀 Creating kind cluster: $CLUSTER_NAME"
kind create cluster --name "$CLUSTER_NAME" --image kindest/node:v1.29.0

echo "📤 Loading image into kind"
kind load docker-image "$IMAGE" --name "$CLUSTER_NAME"

echo "📤 Deploying Kuberhealthy to namespace: $TARGET_NAMESPACE"
kubectl delete namespace "$TARGET_NAMESPACE" --ignore-not-found=true
kustomize build deploy/ | kubectl apply -f -

echo "⏳ Waiting for Kuberhealthy deployment to apply..."
until kubectl get deployment kuberhealthy -n kuberhealthy >/dev/null 2>&1; do
  sleep 1
done
kubectl get events -n kuberhealthy --field-selector involvedObject.kind=Deployment,involvedObject.name=kuberhealthy --sort-by=.lastTimestamp

# Wait for Kuberhealthy pods to be online
kubectl wait --for=condition=Ready pod -l app=kuberhealthy -n "$TARGET_NAMESPACE" --timeout=60s

echo "📄 Tailing Kuberhealthy logs:"
kubectl logs -n "$TARGET_NAMESPACE" -l app=kuberhealthy -f
