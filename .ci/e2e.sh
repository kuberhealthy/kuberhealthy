#!/bin/bash

#####
# This script is created to install kuberhealthy with a few basic checks in a minikube cluster.
# In the long run I hope that we can use it to run test cases.
#####

# Set NS
NS=kuberhealthy

# Create namespace
kubectl create namespace $NS

# Create kuberhealthy crd etc.
kubectl create -f deploy/kuberhealthy.yaml

# Wait for kuberhealthy operator to start
JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'; until kubectl -n $NS get pods -l app=kuberhealthy -o jsonpath="$JSONPATH" 2>&1 |grep -q "Ready=True"; do sleep 1;echo "waiting for kuberhealthy operator to be available"; kubectl get pods -n $NS; done

for i in {1..60}
do
    khsCount=$(kubectl get -n $NS khs -o yaml |grep OK |wc -l)

    if [ $khsCount -eq 3 ]
    then
        echo "Kuberhealthy is working like it should and all tests passed"
        break
    else
        kubectl get -n $NS khs -o yaml
        sleep 10
    fi
done
