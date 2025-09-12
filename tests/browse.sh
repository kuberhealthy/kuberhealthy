#!/usr/bin/env bash
set -euo pipefail

LOCAL_PORT=8080
CLUSTER_NAME=kuberhealthy

# Export the kind context into my local kubectl config
kind export kubeconfig --name ${CLUSTER_NAME}

echo "🌐 Opening http://localhost:${LOCAL_PORT}"
sleep 1 && open "http://localhost:${LOCAL_PORT}" >/dev/null 2>&1 & 

kubectl -n kuberhealthy --context=kind-kuberhealthy port-forward service/kuberhealthy 8080:${LOCAL_PORT}

