# This Justfile is for local development. Releases are done via Github Actions workflows. Just make a tag to cause a release.
IMAGE := "kuberhealthy"
TAG := "localdev"
NOW := `date +%s`

build: # Build Kuberhealthy's image
	IMAGE={{IMAGE}} TAG={{TAG}} bash tests/buildImage.sh

kind: # Run Kuberhealthy locally in a KIND cluster
	bash tests/run-local-kind.sh

kind-clean: # Delete the local KIND cluster
	bash tests/cleanup-kind.sh

test: # Run tests locally
	bash tests/runTests.sh

run: # Run Kuberhealthy locally
	bash tests/runLocal.sh

kustomize: # Apply Kubernetes specs from deploy/ directory
	bash tests/kustomizeApply.sh

browse: # Port-forward Kuberhealthy service and open browser
	bash tests/browse.sh
