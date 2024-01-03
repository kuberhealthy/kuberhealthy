#!/bin/bash

#####
# This script is created to install kuberhealthy with a few basic checks in a minikube cluster.
# In the long run I hope that we can use it to run test cases.
#####

# Set NS
NS=kuberhealthy
name=kuberhealthy

# if unset use "unstable" - useful for local dev testing
IMAGE_URL="$1"
echo "Kuberhealthy image: $IMAGE_URL"

# Create namespace
kubectl create namespace $NS

# wait for kuberhealthy's namespace to get created fully
sleep 2

# Use helm to install kuberhealthy
# the image repository and tag must match the build that just took place
helm install -n $NS --set imageURL=$IMAGE_URL -f .ci/values.yaml  $name deploy/helm/kuberhealthy

# list khchecks
kubectl -n $NS get khc

# let kuberhealthy images boot up
sleep 30

helm ls

echo "get all"
kubectl -n $NS get all
echo "get deployment"
kubectl -n $NS get deployment kuberhealthy -o yaml
echo "get khc"
kubectl -n $NS get khc
echo "get khs:"
kubectl -n $NS get khs

# If the operator dosen't start for some reason kill the test
kubectl -n $NS get pods | grep $name
if [ $? != 0 ]; then
    echo "No Kuberhealthy instance pod found after helm install"
    exit 1
fi

# Wait for kuberhealthy operator to start
kubectl wait --for=condition=Ready pod -l app=kuberhealthy

echo "dump the kuberhealthy deployment logs \n"
kubectl logs -n $NS deployment/kuberhealthy

# repeatedly check for checks to run successfully
for i in {1..20}; do
    khsCount=$(kubectl get -n $NS khs -o yaml | grep "OK: true" | wc -l)
    cDeploy=$(kubectl -n $NS get pods -l app=kuberhealthy-check | grep deployment | grep Completed | wc -l)
    cDNS=$(kubectl -n $NS get pods -l app=kuberhealthy-check | grep dns-status-internal | grep Completed | wc -l)
    cDS=$(kubectl -n $NS get pods -l app=kuberhealthy-check | grep daemonset | grep Completed | wc -l)
    cPR=$(kubectl -n $NS get pods -l app=kuberhealthy-check | grep pod-restarts | grep Completed | wc -l)
    cPS=$(kubectl -n $NS get pods -l app=kuberhealthy-check | grep pod-status | grep Completed | wc -l)

    if [ $khsCount -ge 5 ] && [ $cDeploy -ge 1 ] && [ $cDS -ge 1 ] && [ $cDNS -ge 1 ] && [ $cPR -ge 1 ] && [ $cPS -ge 1 ]; then
        echo "ALL KUBERHEALTHY CHECKS PASSED!!"

		# Print some final output to make debuging easier.
		echo "kuberhealthy logs"
		kubectl logs -n $NS deployment/kuberhealthy
		exit 0 # successful testing
        
    else
        echo "--- Waiting for all kubrhealthy checks to pass...\n"
		echo "Checks Successful of 5: $khsCount"
		echo "Deployment checks completed: $cDeploy"
		echo "DNS checks completed: $cDNS"
		echo "Daemonset checks completed: $cDS"
		echo "Pod Restart checks completed: $cPR"
		echo "Pod Status checks completed: $cPS"
        kubectl get -n $NS pods,khchecks,khstate
        kubectl logs -n $NS -l app=kuberhealthy
        sleep 10
    fi

done

echo "Testing failed due to timeout waiting for successful checks to return."
exit 1 # failed testing due to timeout
