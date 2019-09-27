IMAGE="quay.io/comcast/kuberhealthy"
TAG="2.0.0alpha5"

build:
	docker build -t $(IMAGE):$(TAG) .

push:
	docker push $(IMAGE):$(TAG)

buildExternalChecker:
	docker build -t quay.io/comcast/testexternalcheck:latest -f cmd/testExternalCheck/Dockerfile .

pushExternalChecker:
	docker push quay.io/comcast/testexternalcheck:latest
