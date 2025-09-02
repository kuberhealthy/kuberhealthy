#!/usr/bin/env bash
set -euo pipefail

# Load Kuberhealthy run metadata from environment variables provided to the pod.
UUID="${KH_RUN_UUID:-}"
REPORT_URL="${KH_REPORTING_URL:-}"

if [[ -z "$UUID" || -z "$REPORT_URL" ]]; then
  echo "KH_RUN_UUID and KH_REPORTING_URL must be set" >&2
  exit 1
fi

report_success() {
  curl -sS -X POST \
    -H "Content-Type: application/json" \
    -H "kh-run-uuid: ${UUID}" \
    -d '{"ok":true,"errors":[]}' \
    "${REPORT_URL}"
}

report_failure() {
  local msg="$1"
  curl -sS -X POST \
    -H "Content-Type: application/json" \
    -H "kh-run-uuid: ${UUID}" \
    -d '{"ok":false,"errors":["'"${msg}"'"]}' \
    "${REPORT_URL}"
}

# Add your check logic here. For example purposes, this check reports success
# unless the FAIL environment variable is set to "true".
if [[ "${FAIL:-}" == "true" ]]; then
  report_failure "FAIL was set to true"
else
  report_success
fi
