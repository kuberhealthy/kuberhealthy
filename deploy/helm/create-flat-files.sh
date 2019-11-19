#!/bin/bash

echo "Creating flat kuberhealthy.yaml"
helm template kuberhealthy > ../kuberhealthy.yaml

echo "Creating flat kuberhealthy-prometheus.yaml"
helm template kuberhealthy kuberhealthy --set prometheus.enabled=true prometheus.enableScraping=true prometheus.serviceMonitor=true prometheus.enableAlerting=true > ../kuberhealthy-prometheus.yaml

echo "Creating flat kuberhealthy-prometheus-operator.yaml"
helm template kuberhealthy kuberhealthy --set prometheus.enabled=true prometheus.serviceMonitor=true > ../kuberhealthy-prometheus-operator.yaml