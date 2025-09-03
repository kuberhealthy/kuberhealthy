#!/usr/bin/env bash
set -euo pipefail
[ "${DEBUG:-}" = "1" ] && set -x

IMAGE="${IMAGE:-kuberhealthy:localdev}"
CLUSTER="${CLUSTER:-kuberhealthy-dev}"
NAMESPACE="${NAMESPACE:-kuberhealthy}"

bold() {
  printf "\033[1m%s\033[0m\n" "$*"
}

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing dependency: $1" >&2
    exit 1
  fi
}

bold "Checking prerequisites"
need podman
need kind
need kubectl
if ! command -v kustomize >/dev/null 2>&1; then
  if ! kubectl kustomize >/dev/null 2>&1; then
    echo "kustomize not found" >&2
    exit 1
  fi
fi

bold "Building local image"
podman build -f cmd/kuberhealthy/Podfile -t "$IMAGE" .

bold "Loading image into kind cluster '$CLUSTER'"
TMP_TAR="/tmp/kuberhealthy-image.tar"
podman save "$IMAGE" -o "$TMP_TAR"
kind load image-archive "$TMP_TAR" --name "$CLUSTER"
rm -f "$TMP_TAR"

bold "Applying Kuberhealthy manifests"
if command -v kustomize >/dev/null 2>&1; then
  kustomize build deploy/ | kubectl apply -f -
else
  kubectl kustomize deploy/ | kubectl apply -f -
fi

bold "Restarting deployment"
kubectl -n "$NAMESPACE" rollout restart deployment/kuberhealthy || true

bold "Waiting for rollout"
kubectl -n "$NAMESPACE" rollout status deployment/kuberhealthy --timeout=300s

bold "Listing pods"
kubectl -n "$NAMESPACE" get pods -o wide

bold "Done"
