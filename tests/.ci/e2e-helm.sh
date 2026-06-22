#!/bin/bash
set -euo pipefail

#####
# This script installs kuberhealthy with Helm and runs a sample check in a kind cluster.
#####

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Namespace and deployment name
NS=kuberhealthy
NAME=kuberhealthy

# Image built in CI
IMAGE_URL="$1"
echo "Kuberhealthy image: $IMAGE_URL"

helm upgrade --install "$NAME" "$REPO_ROOT/deploy/helm/kuberhealthy" \
  --namespace "$NS" \
  --create-namespace \
  --set imageURL="$IMAGE_URL" \
  --set deployment.replicas=1 \
  --set deployment.maxSurge=0 \
  --set deployment.maxUnavailable=1 \
  --set service.type=ClusterIP \
  --wait \
  --timeout=180s

capabilities_drop=$(kubectl -n "$NS" get deployment "$NAME" -o go-template='{{range .spec.template.spec.containers}}{{if eq .name "kuberhealthy"}}{{index .securityContext.capabilities.drop 0}}{{end}}{{end}}')
if [ "$capabilities_drop" != "ALL" ]; then
  echo "Expected Helm deployment securityContext.capabilities.drop[0] to be ALL, got: $capabilities_drop"
  exit 1
fi

# Wait for the CRD to be established before creating checks
kubectl wait --for=condition=Established --timeout=60s crd/healthchecks.kuberhealthy.github.io

# Wait for kuberhealthy to be ready
kubectl -n "$NS" rollout status deployment/kuberhealthy

# Create a sample check for testing
kubectl apply -f "$REPO_ROOT/tests/healthcheck-test.yaml"

print_block() {
  printf '[%s] === %s ===\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$1"
  shift
  "$@"
}

print_check_pod_logs() {
  check_pods=$(kubectl -n "$NS" get pods -o name | grep -v "$NAME" || true)
  for pod in $check_pods; do
    print_block "Check Pod Logs ($pod)" kubectl -n "$NS" logs "$pod" || true
  done
}

# repeatedly check for the test check to run successfully
for i in {1..20}; do
    checksOK=$(kubectl get -n "$NS" healthchecks.kuberhealthy.github.io -o jsonpath='{range .items[*]}{.status.ok}{"\n"}{end}' 2>/dev/null | grep -c true || true)
    checksOK=${checksOK//[[:space:]]/}
    completedPods=$(kubectl -n "$NS" get pods --field-selector=status.phase=Succeeded -o name | wc -l | tr -d '[:space:]')

    if [ "$checksOK" -ge 1 ] && [ "$completedPods" -ge 1 ]; then
        echo "ALL KUBERHEALTHY HELM HEALTHCHECKS PASSED!!"
        print_block "Pod List" kubectl -n "$NS" get pods
        print_block "HealthCheck List" kubectl -n "$NS" get healthchecks.kuberhealthy.github.io
        print_block "Kuberhealthy Pod Logs" kubectl -n "$NS" logs deployment/kuberhealthy
        print_check_pod_logs
        exit 0 # successful testing
    else
        echo "--- Waiting for kuberhealthy healthcheck to pass...\n"
        echo "Checks Successful: $checksOK"
        echo "Completed check pods: $completedPods"
        print_block "Pod List" kubectl -n "$NS" get pods
        print_block "HealthCheck List" kubectl -n "$NS" get healthchecks.kuberhealthy.github.io
        print_block "Kuberhealthy Pod Logs" kubectl -n "$NS" logs deployment/kuberhealthy
        print_check_pod_logs
        sleep 10
    fi

done

echo "Testing failed due to timeout waiting for successful Helm checks to return."
exit 1 # failed testing due to timeout
