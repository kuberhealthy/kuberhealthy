#!/bin/bash
# the github action we use has helm 3 (required) as 'helmv3' in its path, so we alias that in and use it if present
if which helmv3; then
    echo "Using helm v3 alias"
    alias helm="helmv3"
fi

helm version
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
