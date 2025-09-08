#!/usr/bin/env bash
set -euo pipefail

LOCAL_PORT="${PORT:-8080}"

echo "🌐 Opening http://localhost:${LOCAL_PORT}"
open "http://localhost:${LOCAL_PORT}" >/dev/null 2>&1 &

