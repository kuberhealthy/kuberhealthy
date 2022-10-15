include ../../Makefile.common.mk

BUILDER := "dockerx-kuberhealthy"
IMAGE := "kuberhealthy/kuberhealthy"
TAG := "unstable"

build-dev:
	docker build -t kuberhealthy:dev -f Dockerfile ../../
test:
	POD_NAME="kuberhealthy-test" go test -run TestWebServer -v -args -- --debug --forceMaster
run:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy --debug --forceMaster
run-influx:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy -debug -forceMaster -enableInflux -influxUrl=http://localhost:8086 -influxDB=kuberhealthy
