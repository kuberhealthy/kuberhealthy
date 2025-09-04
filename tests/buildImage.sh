#!/usr/bin/env bash
set -euo pipefail

IMAGE="${IMAGE:-kuberhealthy}"
TAG="${TAG:-localdev}"

echo "📦 Building Podman image: ${IMAGE}:${TAG}"
podman build -f cmd/kuberhealthy/Podfile -t "${IMAGE}:${TAG}" .

