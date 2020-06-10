# IMAGE="kuberhealthy/kuberhealthy"
# TAG="2.2.0"

build:
<<<<<<< HEAD
	docker build -t kuberhealthy:dev3 -f Dockerfile ../../
=======
	docker build -t kuberhealthy/kuberhealthy:v2.2.1 -f Dockerfile ../../
>>>>>>> 1d9f11c803442a86170a3b9a51ebd3339eefa759

push:
	docker push kuberhealthy/kuberhealthy:v2.2.1

test:
	POD_NAME="kuberhealthy-test" go test -run TestWebServer -v -args -- --debug --forceMaster
run:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy --debug --forceMaster
run-influx:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy -debug -forceMaster -enableInflux -influxUrl=http://localhost:8086 -influxDB=kuberhealthy
