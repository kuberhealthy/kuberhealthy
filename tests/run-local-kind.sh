#!/bin/bash
set -euo pipefail

CLUSTER_NAME="kuberhealthy-dev"
IMAGE="docker.io/kuberhealthy/kuberhealthy:localdev"
TARGET_NAMESPACE="kuberhealthy"
KH_CHECK_REPORT_HOSTNAME="kuberhealthy.${TARGET_NAMESPACE}.svc.cluster.local"
export KIND_EXPERIMENTAL_PROVIDER=podman

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

echo "📦 Building Podman image: $IMAGE"
# this is meant to be run with `just kind` from the root of the repo
podman build -f cmd/kuberhealthy/Podfile -t "$IMAGE" .

# Delete existing cluster
if kind get clusters | grep -q "$CLUSTER_NAME"; then
  echo "🧹 Deleting existing kind cluster: $CLUSTER_NAME"
  kind delete cluster --name "$CLUSTER_NAME"
fi

echo "🚀 Creating kind cluster: $CLUSTER_NAME"
kind create cluster --name "$CLUSTER_NAME" --image kindest/node:v1.29.0

echo "📤 Loading image into kind"
podman save "$IMAGE" -o /tmp/kuberhealthy-image.tar
kind load image-archive /tmp/kuberhealthy-image.tar --name "$CLUSTER_NAME"
rm /tmp/kuberhealthy-image.tar

echo "📤 Deploying Kuberhealthy to namespace: $TARGET_NAMESPACE"
kubectl delete namespace "$TARGET_NAMESPACE" --ignore-not-found=true
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


# Wait for Kuberhealthy pods to be online
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

# if the pod did not come up, but the deployment did, we fetch the logs of the dead pod and exit
if [ "$FOUND_POD" = false ]; then
  echo "‼️ Pod did not appear, running log command for troubleshooting..."
  kubectl logs -n "$TARGET_NAMESPACE" -l app=kuberhealthy # if the pod is not running, this is necessary to get logs
  exit 1
fi

# watch the logs, but if we cant because the pod is crashed, find whatever logs are on the pod
echo "🪵 Tailing Kuberhealthy logs..."
kubectl get pod -n "$TARGET_NAMESPACE"
kubectl logs -n "$TARGET_NAMESPACE" -l app=kuberhealthy -f # this is needed to follow logs for a running pod
