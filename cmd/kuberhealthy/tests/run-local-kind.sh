#!/bin/bash
set -e

# Check for kind
if ! command -v kind >/dev/null 2>&1; then
  echo "❌ 'kind' is not installed. Install it from https://kind.sigs.k8s.io/ before running this script."
  exit 1
fi

CLUSTER_NAME="kuberhealthy-dev"
IMAGE="docker.io/kuberhealthy/kuberhealthy:localdev"
TARGET_NAMESPACE="kuberhealthy"
KH_CHECK_REPORT_URL="http://kuberhealthy.$TARGET_NAMESPACE.svc.cluster.local/externalCheckStatus"

echo "Creating kind cluster: $CLUSTER_NAME"
kind create cluster --name "$CLUSTER_NAME"

echo "Loading image into kind..."
kind load docker-image "$IMAGE" --name "$CLUSTER_NAME"

echo "Deploying Kuberhealthy to local KIND cluster..."
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
  - port: 80
    targetPort: 8080
EOF

echo "Deployment spec applied. Waiting for pod..."

# Wait for pod to be ready
kubectl wait --for=condition=Ready pod -l app=kuberhealthy -n "$TARGET_NAMESPACE" --timeout=60s

# Tail logs
echo "📋 Tailing Kuberhealthy logs:"
kubectl logs -n "$TARGET_NAMESPACE" -l app=kuberhealthy -f