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

deploy-k8s: # Build, load into local kind cluster, apply manifests, restart, list pods
	bash ./tests/deployLocalKind.sh

