IMAGE="kuberhealthy/deployment-check"
TAG="v1.8.2"

build:
	docker build -t ${IMAGE}:${TAG} -f Dockerfile ../../

push:
	docker push ${IMAGE}:${TAG}