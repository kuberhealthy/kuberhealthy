IMAGE="kuberhealthy"
TAG="2.0.0alpha2"

build:
	docker build -t $(IMAGE):$(TAG) .

push:
	docker push $(IMAGE):$(TAG)

buildExternalChecker:
	docker build -t testexternalcheck:latest -f cmd/testExternalCheck/Dockerfile .
