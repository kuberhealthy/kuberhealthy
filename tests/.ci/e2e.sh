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
kubectl wait --for=condition=Established --timeout=60s crd/healthchecks.kuberhealthy.github.io

# Wait for kuberhealthy to be ready
kubectl -n "$NS" rollout status deployment/kuberhealthy

# Create a sample check for testing
kubectl apply -f "$REPO_ROOT/tests/healthcheck-test.yaml"

# print_block prints a header and executes the provided command to make log output easier to parse.
print_block() {
  printf '[%s] === %s ===\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$1"
  shift
  "$@"
}

# print_check_pod_logs iterates over check pods and prints their logs to aid in debugging failures.
print_check_pod_logs() {
  check_pods=$(kubectl -n "$NS" get pods -o name | grep -v "$NAME" || true)
  for pod in $check_pods; do
    print_block "Check Pod Logs ($pod)" kubectl -n "$NS" logs "$pod" || true
  done
}

# repeatedly check for the test check to run successfully
for i in {1..20}; do
    checksOK=$(kubectl get -n "$NS" healthcheck -o jsonpath='{range .items[*]}{.status.ok}{"\n"}{end}' 2>/dev/null | grep -c true || true)
    checksOK=${checksOK//[[:space:]]/}
    completedPods=$(kubectl -n "$NS" get pods --field-selector=status.phase=Succeeded -o name | wc -l | tr -d '[:space:]')

    if [ "$checksOK" -ge 1 ] && [ "$completedPods" -ge 1 ]; then
        echo "ALL KUBERHEALTHY CHECKS PASSED!!"
        print_block "Pod List" kubectl -n "$NS" get pods
        print_block "HealthCheck List" kubectl -n "$NS" get healthcheck
        print_block "Kuberhealthy Pod Logs" kubectl -n "$NS" logs deployment/kuberhealthy
        print_check_pod_logs
        exit 0 # successful testing
    else
        echo "--- Waiting for kuberhealthy check to pass...\n"
        echo "Checks Successful: $checksOK"
        echo "Completed check pods: $completedPods"
        print_block "Pod List" kubectl -n "$NS" get pods
        print_block "HealthCheck List" kubectl -n "$NS" get healthcheck
        print_block "Kuberhealthy Pod Logs" kubectl -n "$NS" logs deployment/kuberhealthy
        print_check_pod_logs
        sleep 10
    fi

done

echo "Testing failed due to timeout waiting for successful checks to return."
exit 1 # failed testing due to timeout
