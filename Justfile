# This Justfile is for local development. Releases are done via Github Actions workflows. Just make a tag to cause a release.
IMAGE := "docker.io/kuberhealthy/kuberhealthy"
TAG := "localdev"
NOW := `date +%s`

build: # Build Kuberhealthy's image
	podman build -f cmd/kuberhealthy/Containerfile -t {{IMAGE}}:{{TAG}} .

kind: # Run Kuberhealthy locally in a KIND cluster
	./tests/run-local-kind.sh

test: # Run tests locally
	go test -v internal/...
	# go test -v pkg/... # uncomment when tests exist here
	go test -v cmd/...

run: # Run Kuberhealthy locally
	cd cmd/kuberhealthy && \
	go build -v && \
	KH_EXTERNAL_REPORTING_URL=localhost:8006 POD_NAMESPACE=kuberhealthy POD_NAME="kuberhealthy-test" ./kuberhealthy --debug --config ./test/test-config.yaml

kustomize: # Apply Kubernetes specs from deploy/ directory
	kustomize build deploy/ | kubectl apply -f -

deploy-k8s1: # Build, ship to k8s1 via scp, import into containerd, apply manifests, restart, list pods
	bash ./hack/push-to-node.sh
