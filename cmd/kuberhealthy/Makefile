BUILDER := "dockerx-kuberhealthy"
IMAGE := "kuberhealthy/kuberhealthy"
TAG := "unstable"

.PHONY: test build build-dev push run run-influx

.PHONY: build build-dev push test run run-influx

build:
	docker buildx build linux/amd64,linux/arm64 --no-cache --pull -t ${IMAGE}:${TAG} -f Dockerfile ../../
build-dev:
	docker buildx build linux/amd64,linux/arm64 -t ${IMAGE}:${TAG} -f Dockerfile ../../
test:
	POD_NAME="kuberhealthy-test" go test -run TestWebServer -v -args -- --debug --forceMaster
run:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy --debug --forceMaster
run-influx:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy -debug -forceMaster -enableInflux -influxUrl=http://localhost:8086 -influxDB=kuberhealthy
