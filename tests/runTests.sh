#!/usr/bin/env bash
set -euo pipefail

go test -v internal/...
# go test -v pkg/... # uncomment when tests exist here
go test -v cmd/...

