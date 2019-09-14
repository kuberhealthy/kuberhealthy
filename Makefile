IMAGE="kuberhealthy"
TAG="local"

build:
	docker build -t $(IMAGE):$(TAG) .

push:
	docker push $(IMAGE):$(TAG)

buildExternalChecker:
	docker build -t testexternalcheck:latest -f cmd/testExternalCheck/Dockerfile .
