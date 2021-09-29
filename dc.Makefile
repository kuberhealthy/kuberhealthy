all: build.docker 
build.docker:
	  docker build -t docker-hub-remote.dr.corp.adobe.com/kuberhealthy/deployment-check:v1.9.0-dc -f cmd/deployment-check/dc.Dockerfile .
