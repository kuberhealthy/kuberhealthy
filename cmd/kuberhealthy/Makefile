IMAGE="kuberhealthy/kuberhealthy"
TAG="iss-870"

build:
	docker build --no-cache --pull -t ${IMAGE}:${TAG} -f Dockerfile ../../
build-dev:
	docker build -t kuberhealthy:dev -f Dockerfile ../../
push:
	docker push ${IMAGE}:${TAG}
test:
	POD_NAME="kuberhealthy-test" go test -run TestWebServer -v -args -- --debug --forceMaster
run:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy --debug --forceMaster
run-influx:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy -debug -forceMaster -enableInflux -influxUrl=http://localhost:8086 -influxDB=kuberhealthy
