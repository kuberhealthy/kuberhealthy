#!/bin/bash
set -e

CLUSTER_NAME="kuberhealthy-dev"
IMAGE="docker.io/kuberhealthy/kuberhealthy:localdev"
TARGET_NAMESPACE="kuberhealthy"
KH_CHECK_REPORT_URL="http://kuberhealthy.$TARGET_NAMESPACE.svc.cluster.local/externalCheckStatus"
K3S_CONTAINER_NAME="k3s-server"
KUBECONFIG_DIR="$HOME/.k3s"
KUBECONFIG="$KUBECONFIG_DIR/kubeconfig.yaml"

# Check for a container engine
if command -v podman >/dev/null 2>&1; then
  CONTAINER_ENGINE="podman"
elif command -v docker >/dev/null 2>&1; then
  CONTAINER_ENGINE="docker"
else
  echo "❌ Neither podman nor docker found"
  exit 1
fi

echo "✅ Using container engine: $CONTAINER_ENGINE"

# Remove any existing container
if $CONTAINER_ENGINE ps -a --format '{{.Names}}' | grep -qx "$K3S_CONTAINER_NAME"; then
  echo "🧹 Removing existing k3s container..."
  $CONTAINER_ENGINE rm -f "$K3S_CONTAINER_NAME"
fi

# Start new k3s container
echo "🚀 Starting new k3s container..."
mkdir -p "$KUBECONFIG_DIR"
$CONTAINER_ENGINE run -d --name "$K3S_CONTAINER_NAME" \
  --privileged \
  -p 6443:6443 \
  -e K3S_KUBECONFIG_OUTPUT=/output/kubeconfig.yaml \
  -e K3S_KUBECONFIG_MODE=666 \
  -v k3s-data:/var/lib/rancher/k3s \
  -v "$KUBECONFIG_DIR":/output \
  docker.io/rancher/k3s:v1.28.2-k3s1 server \
    --disable-cpuset \
    --kubelet-arg="cgroup-driver=none" \
    --kubelet-arg="--cgroups-per-qos=false" \
    --kubelet-arg="--enforce-node-allocatable="

# Wait for kubeconfig and API server
echo "⏳ Waiting for API server to respond..."
for i in {1..30}; do
  if [ -f "$KUBECONFIG" ] && kubectl --kubeconfig="$KUBECONFIG" get --raw /healthz >/dev/null 2>&1; then
    echo "✅ API server is healthy"
    break
  fi
  sleep 1
done
[ -f "$KUBECONFIG" ] || { echo "❌ kubeconfig not found"; exit 1; }
kubectl get --raw /healthz >/dev/null 2>&1 || {
  echo "❌ Kubernetes API server is not responding after 30s"
  exit 1
}

export KUBECONFIG

# Deploy Kuberhealthy
echo "📦 Deploying Kuberhealthy into k3s..."
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

echo "📋 Tailing Kuberhealthy logs:"
kubectl logs -n "$TARGET_NAMESPACE" -l app=kuberhealthy -f
