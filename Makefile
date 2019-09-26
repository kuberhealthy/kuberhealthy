IMAGE="quay.io/comcast/kuberhealthy"
TAG="2.0.0alpha4"

build:
	docker build -t $(IMAGE):$(TAG) .

push:
	docker push $(IMAGE):$(TAG)

buildExternalChecker:
	docker build -t testexternalcheck:latest -f cmd/testExternalCheck/Dockerfile .
