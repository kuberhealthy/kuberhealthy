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

# Create kuberhealthy crd etc.
kubectl create -f .ci/kuberhealthy-e2e.yaml

kubectl -n $NS get khc

sleep 90

echo "get all \n"
kubectl -n $NS get all
echo "get khc  \n"
kubectl -n $NS get khc
echo "get khs \n"
kubectl -n $NS get khs

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

    if [ $khsCount -ge 3 ] && [ $cDeploy -ge 1 ] && [ $cDS -ge 1 ] && [ $cDNS -ge 1 ]
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
done

# Print some final output to make debuging easier.
kubectl logs -n $NS --selector $selector

echo "get khs \n"
kubectl get -n $NS khs -o yaml

echo "get all \n"
kubectl get -n $NS all

echo "completed deployment logs"
oc -n $NS logs $(oc get pod -n $NS -l kuberhealthy-check-name=deployment |grep Completed |tail -1 |awk '{print $1}')

echo "Error deployment logs"
oc -n $NS logs $(oc get pod -n $NS -l kuberhealthy-check-name=deployment |grep Error |tail -1 |awk '{print $1}')
