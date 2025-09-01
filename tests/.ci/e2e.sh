#!/bin/bash
set -euo pipefail

#####
# This script installs kuberhealthy and runs a sample check in a kind cluster.
#####

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Namespace and deployment name
NS=kuberhealthy
NAME=kuberhealthy

# Image built in CI
IMAGE_URL="$1"
echo "Kuberhealthy image: $IMAGE_URL"

# Create namespace for kuberhealthy
kubectl create namespace "$NS"

# Install kuberhealthy using kustomize
kubectl apply -k "$REPO_ROOT/deploy"

# Update the deployment to use the built image
kubectl -n "$NS" set image deployment/kuberhealthy kuberhealthy="$IMAGE_URL"

# Wait for the CRD to be established before creating checks
kubectl wait --for=condition=Established --timeout=60s crd/kuberhealthychecks.kuberhealthy.github.io

# Wait for kuberhealthy to be ready
kubectl -n "$NS" rollout status deployment/kuberhealthy

# Create a sample check for testing
kubectl apply -f "$REPO_ROOT/tests/khcheck-test.yaml"

print_block() {
  printf '[%s] === %s ===\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$1"
  shift
  "$@"
}

# repeatedly check for the test check to run successfully
for i in {1..20}; do
    checksOK=$(kubectl get -n "$NS" kuberhealthycheck -o jsonpath='{range .items[*]}{.status.ok}{"\n"}{end}' 2>/dev/null | grep -c true || echo 0)
    completedPods=$(kubectl -n "$NS" get pods --field-selector=status.phase=Succeeded -o name | wc -l)

    if [ "$checksOK" -ge 1 ] && [ "$completedPods" -ge 1 ]; then
        echo "ALL KUBERHEALTHY CHECKS PASSED!!"
        print_block "Pod List" kubectl -n "$NS" get pods
        print_block "KuberhealthyCheck List" kubectl -n "$NS" get kuberhealthycheck
        print_block "Kuberhealthy Pod Logs" kubectl -n "$NS" logs deployment/kuberhealthy
        exit 0 # successful testing
    else
        echo "--- Waiting for kuberhealthy check to pass...\n"
        echo "Checks Successful: $checksOK"
        echo "Completed check pods: $completedPods"
        print_block "Pod List" kubectl -n "$NS" get pods
        print_block "KuberhealthyCheck List" kubectl -n "$NS" get kuberhealthycheck
        print_block "Kuberhealthy Pod Logs" kubectl -n "$NS" logs deployment/kuberhealthy
        sleep 10
    fi

done

echo "Testing failed due to timeout waiting for successful checks to return."
exit 1 # failed testing due to timeout
