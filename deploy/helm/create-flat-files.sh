#!/bin/bash

echo "Creating flat kuberhealthy.yaml"
helm template --namespace kuberhealthy kuberhealthy > ../kuberhealthy.yaml

echo "Creating flat kuberhealthy-prometheus.yaml"
helm template --namespace kuberhealthy kuberhealthy kuberhealthy --set prometheus.enabled=true --set prometheus.enableScraping=true --set prometheus.enableAlerting=true > ../kuberhealthy-prometheus.yaml

echo "Creating flat kuberhealthy-prometheus-operator.yaml"
helm template --namespace kuberhealthy kuberhealthy kuberhealthy --set prometheus.enabled=true --set prometheus.enableScraping=true --set prometheus.enableAlerting=true --set prometheus.serviceMonitor=true > ../kuberhealthy-prometheus-operator.yaml
