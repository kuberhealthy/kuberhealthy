#!/usr/bin/env bash
set -euo pipefail

export KIND_EXPERIMENTAL_PROVIDER=podman
CLUSTER_NAME="kuberhealthy"

echo "🧹 Deleting kind cluster: ${CLUSTER_NAME}"
kind delete cluster --name "${CLUSTER_NAME}"

# Remove the dedicated kubeconfig used by the kind script if it exists
KUBECONFIG_PATH="$(pwd)/tests/kubeconfig.kind.${CLUSTER_NAME}"
if [[ -f "${KUBECONFIG_PATH}" ]]; then
  rm -f "${KUBECONFIG_PATH}"
fi
