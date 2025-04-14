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
kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: $TARGET_NAMESPACE
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kuberhealthy
  namespace: $TARGET_NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kuberhealthy
  template:
    metadata:
      labels:
        app: kuberhealthy
    spec:
      containers:
      - name: kuberhealthy
        image: $IMAGE
        imagePullPolicy: Never
        env:
        - name: TARGET_NAMESPACE
          value: "$TARGET_NAMESPACE"
        - name: KH_CHECK_REPORT_URL
          value: "$KH_CHECK_REPORT_URL"
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: kuberhealthy
  namespace: $TARGET_NAMESPACE
spec:
  selector:
    app: kuberhealthy
  ports:
  - port: 8080
    targetPort: 8080
EOF

echo "⏳ Waiting for Kuberhealthy pod..."
kubectl wait --for=condition=Ready pod -l app=kuberhealthy -n "$TARGET_NAMESPACE" --timeout=60s

echo "📄 Tailing Kuberhealthy logs:"
kubectl logs -n "$TARGET_NAMESPACE" -l app=kuberhealthy -f
