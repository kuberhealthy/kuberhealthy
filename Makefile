IMAGE="quay.io/comcast/kuberhealthy"
TAG="unstable"

build:
	docker build -t $(IMAGE):$(TAG) .

push:
docker push $(IMAGE):$(TAG)
