#!/usr/bin/env bash
set -euo pipefail

pushd cmd/kuberhealthy >/dev/null
go build -v
popd >/dev/null

KH_LOG_LEVEL=debug POD_NAMESPACE=kuberhealthy POD_NAME="kuberhealthy-test" ./cmd/kuberhealthy/kuberhealthy

