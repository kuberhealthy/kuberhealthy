#!/bin/bash

echo "Creating flat kuberhealthy.yaml"
helm template --namespace kuberhealthy kuberhealthy > ../kuberhealthy.yaml

echo "Creating flat kuberhealthy-prometheus.yaml"
helm template --namespace kuberhealthy kuberhealthy kuberhealthy --set prometheus.enabled=true --set prometheus.enableScraping=true --set prometheus.enableAlerting=true > ../kuberhealthy-prometheus.yaml

echo "Creating flat kuberhealthy-prometheus-operator.yaml"
helm template --namespace kuberhealthy kuberhealthy kuberhealthy --set prometheus.enabled=true --set prometheus.enableScraping=true --set prometheus.enableAlerting=true --set prometheus.serviceMonitor=true > ../kuberhealthy-prometheus-operator.yaml


# temp helm fix described in issue #279:

cp -f ../kuberhealthy.yaml kuberhealthy.yaml.old
cat kuberhealthy/crds/* > ../kuberhealthy.yaml
cat kuberhealthy.yaml.old >> ../kuberhealthy.yaml

cp -f ../kuberhealthy-prometheus.yaml kuberhealthy-prometheus.yaml.old
cat kuberhealthy/crds/* > ../kuberhealthy-prometheus.yaml
cat kuberhealthy-prometheus.yaml.old >> ../kuberhealthy-prometheus.yaml

cp -f ../kuberhealthy-prometheus-operator.yaml kuberhealthy-prometheus-operator.yaml.old
cat kuberhealthy/crds/* > ../kuberhealthy-prometheus-operator.yaml
cat kuberhealthy-prometheus-operator.yaml.old >> ../kuberhealthy-prometheus-operator.yaml
