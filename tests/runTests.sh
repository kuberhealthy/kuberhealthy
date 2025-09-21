#!/usr/bin/env bash
set -euo pipefail


cd cmd/kuberhealthy
go test -v ./...
cd ../../
cd internal
go test -v ./...
