IMAGE="quay.io/comcast/kuberhealthy"
TAG="integrii"

build:
	docker build -t $(IMAGE):$(TAG) .

push:
	docker push $(IMAGE):$(TAG)


buildExternalChecker:
	docker build -t testexternalcheck:latest -f cmd/testExternalCheck/Dockerfile .