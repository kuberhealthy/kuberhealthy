test:
	POD_NAME="kuberhealthy-test" go test -run TestWebServer -v -args -debug -forceMaster
run:
	go build
	POD_NAME="kuberhealthy-test" ./kuberhealthy -debug -forceMaster
