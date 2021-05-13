IMAGE="kuberhealthy/deployment-check"
TAG="v1.9.0"

build:
	docker build -t ${IMAGE}:${TAG} -f Dockerfile ../../

push:
	docker push ${IMAGE}:${TAG}