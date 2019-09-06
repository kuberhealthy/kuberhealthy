IMAGE="quay.io/comcast/kuberhealthy"
TAG="integrii"

build:
	docker build -t $(IMAGE):$(TAG) .

push:
docker push $(IMAGE):$(TAG)
