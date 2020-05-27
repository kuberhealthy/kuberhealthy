#!/bin/bash

#####
# This script is created to install kuberhealthy with a few basic checks in a minikube cluster.
# In the long run I hope that we can use it to run test cases.
#####

# Set NS
NS=kuberhealthy
name=kuberhealthy

# Create namespace
kubectl create namespace $NS

# Sometimes the kuberhealthy resources get's created...
sleep 2

# Use helm to install kuberhealthy
# the image repository and tag must match the build that just took place
helm install -n $NS --set global.image.repository=kuberhealthy,global.image.tag=$GITHUB_RUN_ID -f .ci/values.yaml  $name deploy/helm/kuberhealthy

kubectl -n $NS get khc

sleep 90

helm ls

echo "get all \n"
kubectl -n $NS get all
echo "get khc  \n"
kubectl -n $NS get khc
echo "get khs \n"
kubectl -n $NS get khs

# If the operator dosen't start for some reason kill the test
kubectl -n $NS get pods |grep $name
if [ $? != 0 ]
then
    echo "No operator pod found"
    exit 1
fi

# Wait for kuberhealthy operator to start
JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'; until kubectl -n $NS get pods -l app=kuberhealthy -o jsonpath="$JSONPATH" 2>&1 |grep -q "Ready=True"; do sleep 1;echo "waiting for kuberhealthy operator to be available"; kubectl get pods -n $NS; done

echo "get deployment logs \n"
selfLink=$(kubectl get deployment.apps $name -n $NS -o jsonpath={.metadata.selfLink})
selector=$(kubectl -n $NS get --raw "${selfLink}/scale" | jq -r .status.selector)
kubectl logs -n $NS --selector $selector

# Verify that the khc went as they should.
for i in {1..60}
do
    khsCount=$(kubectl get -n $NS khs -o yaml |grep "OK: true" |wc -l)
    cDeploy=$(kubectl -n $NS get pods -l app=kuberhealthy-check |grep deployment |grep Completed |wc -l)
    cDNS=$(kubectl -n $NS get pods -l app=kuberhealthy-check |grep dns-status-internal |grep Completed |wc -l)
    cDS=$(kubectl -n $NS get pods -l app=kuberhealthy-check |grep daemonset |grep Completed |wc -l)
    cPR=$(kubectl -n $NS get pods -l app=kuberhealthy-check |grep pod-restarts |grep Completed |wc -l)
    cPS=$(kubectl -n $NS get pods -l app=kuberhealthy-check |grep pod-status |grep Completed |wc -l)
    failCount=$(kubectl get -n $NS khs -o yaml |grep "OK: false" |wc -l)

    if [ $khsCount -ge 5 ] && [ $cDeploy -ge 1 ] && [ $cDS -ge 1 ] && [ $cDNS -ge 1 ] && [ $cPR -ge 1 ] && [ $cPS -ge 1 ]
    then
        echo "Kuberhealthy is working like it should and all tests passed"
        break
    else
        echo "\n"
        kubectl get -n $NS pods
        sleep 10
        echo "\n"
        kubectl get -n $NS khs -o yaml
    fi

    if [ $failCount -ge 1 ]
    then
        echo "Kuberhealthy check failed"
        exit 1
    fi

done

# Print some final output to make debuging easier.
echo "kuberhealthy logs"
kubectl logs -n $NS --selector $selector

echo "get khs \n"
kubectl get -n $NS khs -o yaml

echo "get all \n"
kubectl get -n $NS all

# Checking for Completed and Error nodes
kubectl -n $NS get pods |grep deployment |grep -q Completed
if [ $? == 0 ]
then
    echo "completed deployment logs"
    kubectl -n $NS logs $(kubectl get pod -n $NS -l kuberhealthy-check-name=deployment |grep Completed |tail -1 |awk '{print $1}')
else
    echo "No Completed deployment pods found"
fi

kubectl -n $NS get pods |grep deployment |grep -q Error
if [ $? == 0 ]
then
    echo "Error deployment logs"
    kubectl -n $NS logs $(kubectl get pod -n $NS -l kuberhealthy-check-name=deployment |grep Error |tail -1 |awk '{print $1}')
else
    echo "No Error deployment pods found"
fi
