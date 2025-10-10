#!/usr/bin/env bash
set -euo pipefail

kustomize build deploy/kustomize/ | kubectl apply -f -
