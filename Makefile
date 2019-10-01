IMAGE="quay.io/comcast/kuberhealthy"
TAG="2.0.0alpha5"

build:
	docker build -t $(IMAGE):$(TAG) .

push:
	docker push $(IMAGE):$(TAG)

external: buildExternalChecker pushExternalChecker

buildExternalChecker:
	docker build -t quay.io/comcast/test-external-check:latest -f cmd/test-external-check/Dockerfile .

pushExternalChecker:
	docker push quay.io/comcast/test-external-check:latest
