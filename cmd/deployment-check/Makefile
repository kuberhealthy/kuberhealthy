IMAGE="kuberhealthy/deployment-check"
TAG="v1.7.1"

build:
	docker build -t ${IMAGE}:${TAG} -f Dockerfile ../../

push:
	docker push ${IMAGE}:${TAG}