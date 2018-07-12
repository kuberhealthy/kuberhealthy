#!/bin/bash
# Use to test the helm chart when making changes.

# if we enable prometheus, then it requires the target helm to have 
# the type apiVersion "monitoring.coreos.com/v1" available.
helm install --dry-run --debug --set prometheus.enableAlerting=false,prometheus.enableScraping=false .
