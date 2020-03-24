# IMAGE="kuberhealthy/kuberhealthy"
# TAG="2.1.0"

build:
	docker build -t kuberhealthy/kuberhealthy:2.1.0 -f Dockerfile ../../

push:
	docker push kuberhealthy/kuberhealthy:2.1.0

test:
	POD_NAME="kuberhealthy-test" go test -run TestWebServer -v -args -- --debug --forceMaster
run:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy --debug --forceMaster
run-influx:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy -debug -forceMaster -enableInflux -influxUrl=http://localhost:8086 -influxDB=kuberhealthy
