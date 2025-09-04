#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="kuberhealthy"
SERVICE="svc/kuberhealthy"
LOCAL_PORT="${PORT:-8080}"

echo "🔌 Port-forwarding ${SERVICE} in namespace ${NAMESPACE} to localhost:${LOCAL_PORT}"
echo "Press Ctrl-C to exit. Browser opens only once."

OPENED=0
while true; do
  if [[ "$OPENED" -eq 0 ]]; then
    ( sleep 1; echo "🌐 Opening http://localhost:${LOCAL_PORT}"; open "http://localhost:${LOCAL_PORT}" >/dev/null 2>&1 ) &
    OPENED=1
  fi
  set +e
  kubectl -n "${NAMESPACE}" port-forward "${SERVICE}" "${LOCAL_PORT}:80"
  RC=$?
  set -e
  if [[ $RC -eq 130 || $RC -eq 143 ]]; then
    echo "Interrupted, exiting."
    exit $RC
  fi
  echo "Port-forward ended (code $RC). Retrying in 1s..."
  sleep 1
done

