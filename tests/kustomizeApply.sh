#!/usr/bin/env bash
set -euo pipefail

kustomize build deploy/ | kubectl apply -f -

