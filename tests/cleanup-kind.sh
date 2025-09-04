#!/usr/bin/env bash
set -euo pipefail

export KIND_EXPERIMENTAL_PROVIDER=podman
CLUSTER_NAME="kuberhealthy-dev"

echo "🧹 Deleting kind cluster: ${CLUSTER_NAME}"
kind delete cluster --name "${CLUSTER_NAME}"

