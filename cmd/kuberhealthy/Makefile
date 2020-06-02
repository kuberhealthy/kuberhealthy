# IMAGE="kuberhealthy/kuberhealthy"
# TAG="2.2.0"

build:
	docker build -t kuberhealthy/kuberhealthy:v2.2.1-test -f Dockerfile ../../

push:
	docker push kuberhealthy/kuberhealthy:v2.2.1-test

test:
	POD_NAME="kuberhealthy-test" go test -run TestWebServer -v -args -- --debug --forceMaster
run:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy --debug --forceMaster
run-influx:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy -debug -forceMaster -enableInflux -influxUrl=http://localhost:8086 -influxDB=kuberhealthy
