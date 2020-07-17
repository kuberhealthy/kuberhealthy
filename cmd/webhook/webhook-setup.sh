#!/bin/bash

echo "Retrieving certificate authority data from cluster: $(kubectl config current-context)"
CA_DATA=$(kubectl config view --raw --minify --flatten -o jsonpath='{.clusters[].cluster.certificate-authority-data}') && echo "CA_DATA set to $CA_DATA"
