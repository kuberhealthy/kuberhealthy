#!/bin/bash

#####
# This script is created to install kuberhealthy with a few basic checks in a minikube cluster.
# In the long run I hope that we can use it to run test cases.
#####

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Namespace and deployment name
NS=kuberhealthy
name=kuberhealthy

# Image built in CI
IMAGE_URL="$1"
echo "Kuberhealthy image: $IMAGE_URL"

# Create namespace for kuberhealthy
kubectl create namespace "$NS"

# Ensure namespace is fully created
sleep 2

# Install kuberhealthy using kustomize
kubectl apply -k "$REPO_ROOT/deploy"

# Update the deployment to use the built image
kubectl -n "$NS" set image deployment/kuberhealthy kuberhealthy="$IMAGE_URL"

# Create a sample check for testing
kubectl apply -f "$REPO_ROOT/tests/khcheck-test.yaml"

# Allow kuberhealthy components to start
sleep 30

echo "get all"
kubectl -n "$NS" get all
echo "get deployment"
kubectl -n "$NS" get deployment kuberhealthy -o yaml
echo "get checks"
kubectl -n "$NS" get kuberhealthycheck

# If the operator doesn't start for some reason kill the test
kubectl -n "$NS" get pods | grep "$name"
if [ $? != 0 ]; then
    echo "No Kuberhealthy instance pod found after install"
    exit 1
fi

# Wait for kuberhealthy operator to start
kubectl wait --for=condition=Ready pod -l app=kuberhealthy

echo "dump the kuberhealthy deployment logs \n"
kubectl logs -n "$NS" deployment/kuberhealthy

# repeatedly check for the test check to run successfully
for i in {1..20}; do
    checksOK=$(kubectl get -n "$NS" kuberhealthycheck -o jsonpath='{range .items[*]}{.status.ok}{"\n"}{end}' | grep -c true)
    completedPods=$(kubectl -n "$NS" get pods --field-selector=status.phase=Succeeded -o name | wc -l)

    if [ "$checksOK" -ge 1 ] && [ "$completedPods" -ge 1 ]; then
        echo "ALL KUBERHEALTHY CHECKS PASSED!!"

        # Print some final output to make debugging easier.
        echo "kuberhealthy logs"
        kubectl logs -n "$NS" deployment/kuberhealthy
        exit 0 # successful testing

    else
        echo "--- Waiting for kuberhealthy check to pass...\n"
        echo "Checks Successful: $checksOK"
        echo "Completed check pods: $completedPods"
        kubectl get -n "$NS" pods,kuberhealthychecks
        kubectl logs -n "$NS" deployment/kuberhealthy
        sleep 10
    fi

done

echo "Testing failed due to timeout waiting for successful checks to return."
exit 1 # failed testing due to timeout
