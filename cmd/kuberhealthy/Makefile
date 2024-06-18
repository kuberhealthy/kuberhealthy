BUILDER := "dockerx-kuberhealthy"
IMAGE := "kuberhealthy"
TAG := "unstable"

.PHONY: test build build-dev push run run-influx

.PHONY: build build-dev push test run run-influx

kind:
	podman build -t kuberhealthy:unstable -f Dockerfile ../../
	#docker build -t ${IMAGE}:${TAG} -f Dockerfile ../../
	@-kind delete cluster --name kuberhealthy
	kind create cluster --name kuberhealthy
	podman save -o kuberhealthy.tar kuberhealthy:unstable
	kind load image-archive kuberhealthy.tar --name kuberhealthy
	kubectl apply --context=kind-kuberhealthy -f ../../deploy/kuberhealthy.yaml
	kubectl --context=kind-kuberhealthy -n kuberhealthy set image deployment/kuberhealthy kuberhealthy=kuberhealthy:unstable
build:
	docker buildx build --platform linux/amd64,linux/arm64 --push -t ${IMAGE}:${TAG} -f Dockerfile ../../
push:
	docker push ${IMAGE}:${TAG}
test:
	POD_NAME="kuberhealthy-test" go test -v -args -- --debug --forceMaster
run:
	go build
	KH_EXTERNAL_REPORTING_URL=localhost:8006 POD_NAMESPACE=kuberhealthy POD_NAME="kuberhealthy-test" ./kuberhealthy --debug --forceMaster --config ./test/test-config.yaml
