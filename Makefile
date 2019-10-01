IMAGE="quay.io/comcast/kuberhealthy"
TAG="2.0.0alpha5"

build:
	docker build -t $(IMAGE):$(TAG) .

push:
	docker push $(IMAGE):$(TAG)

buildExternalChecker:
	docker build -t integrii/kh-test-check:latest -f cmd/testExternalCheck/Dockerfile .

pushExternalChecker:
	docker push quay.io/comcast/test-external-check:latest
