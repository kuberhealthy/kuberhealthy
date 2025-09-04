#!/usr/bin/env bash
set -euo pipefail

pushd cmd/kuberhealthy >/dev/null
go build -v
popd >/dev/null

KH_LOG_LEVEL=debug \
KH_EXTERNAL_REPORTING_URL=localhost:80 \
POD_NAMESPACE=kuberhealthy \
POD_NAME="kuberhealthy-test" \
./cmd/kuberhealthy/kuberhealthy

