test:
	MY_POD_NAME="kuberhealthy-test" go test -run TestWebServer -v -args -debug -forceMaster
run:
	go build
	MY_POD_NAME="kuberhealthy-test" ./kuberhealthy -debug -forceMaster
