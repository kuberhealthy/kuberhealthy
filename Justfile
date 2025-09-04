# This Justfile is for local development. Releases are done via Github Actions workflows. Just make a tag to cause a release.
IMAGE := "kuberhealthy"
TAG := "localdev"
NOW := `date +%s`

build: # Build Kuberhealthy's image
	podman build -f cmd/kuberhealthy/Podfile -t {{IMAGE}}:{{TAG}} .

kind: # Run Kuberhealthy locally in a KIND cluster
	./tests/run-local-kind.sh

test: # Run tests locally
	go test -v internal/...
	# go test -v pkg/... # uncomment when tests exist here
	go test -v cmd/...

run: # Run Kuberhealthy locally
	cd cmd/kuberhealthy && \
	go build -v && \
	cd ../.. && \
	KH_LOG_LEVEL=debug KH_EXTERNAL_REPORTING_URL=localhost:80 POD_NAMESPACE=kuberhealthy POD_NAME="kuberhealthy-test" ./cmd/kuberhealthy/kuberhealthy

kustomize: # Apply Kubernetes specs from deploy/ directory
	kustomize build deploy/ | kubectl apply -f -

browse: # Port-forward Kuberhealthy service and open browser
    #!/usr/bin/env bash
    set -euo pipefail
    NAMESPACE="kuberhealthy"
    SERVICE="svc/kuberhealthy"
    LOCAL_PORT="${PORT:-8080}"
    echo "🔌 Port-forwarding ${SERVICE} in namespace ${NAMESPACE} to localhost:${LOCAL_PORT}"
    # Ensure the service exists before trying to forward
    kubectl -n "${NAMESPACE}" get "${SERVICE}" >/dev/null
    # Open browser shortly after port-forward starts
    ( sleep 1; echo "🌐 Opening http://localhost:${LOCAL_PORT}"; open "http://localhost:${LOCAL_PORT}" >/dev/null 2>&1 ) &
    # Hold the port-forward in the foreground until interrupted
    kubectl -n "${NAMESPACE}" port-forward "${SERVICE}" "${LOCAL_PORT}:80"
