#!/bin/bash

echo "Creating flat kuberhealthy.yaml"
helm template kuberhealthy > ../kuberhealthy.yaml

echo "Creating flat kuberhealthy-prometheus.yaml"
helm template kuberhealthy kuberhealthy --set global.prometheus.enabled=true global.prometheus.enableScraping=true global.prometheus.serviceMonitor=true global.prometheus.enableAlerting=true > ../kuberhealthy-prometheus.yaml

echo "Creating flat kuberhealthy-prometheus-operator.yaml"
helm template kuberhealthy kuberhealthy --set global.prometheus.enabled=true global.prometheus.serviceMonitor=true > ../kuberhealthy-prometheus-operator.yaml
