test:
	GOCACHE=off MY_POD_NAME="kuberhealthy-test" go test -run TestWebServer -args -v -debug -forceMaster
run:
	go build
	MY_POD_NAME="kuberhealthy-test" ./kuberhealthy -debug -forceMaster
