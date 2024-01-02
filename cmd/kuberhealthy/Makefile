BUILDER := "dockerx-kuberhealthy"
IMAGE := "kuberhealthy/kuberhealthy"
TAG := "unstable"

.PHONY: test build build-dev push run run-influx

.PHONY: build build-dev push test run run-influx

build-dev:
	docker build -t ${IMAGE}:${TAG} -f Dockerfile ../../
build:
	docker buildx build --platform linux/amd64,linux/arm64 --push -t ${IMAGE}:${TAG} -f Dockerfile ../../
push:
	docker push ${IMAGE}:${TAG}
test:
	POD_NAME="kuberhealthy-test" go test -run TestWebServer -v -args -- --debug --forceMaster
run:
	go build
	KH_EXTERNAL_REPORTING_URL=localhost:8006 POD_NAMESPACE=kuberhealthy POD_NAME="kuberhealthy-test" ./kuberhealthy --debug --forceMaster --config ./test/test-config.yaml
run-influx:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy -debug -forceMaster -enableInflux -influxUrl=http://localhost:8086 -influxDB=kuberhealthy
