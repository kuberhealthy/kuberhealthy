IMAGE="quay.io/comcast/kuberhealthy"
TAG="2.0.0alpha5"

build:
	docker build -t $(IMAGE):$(TAG) .

push:
	docker push $(IMAGE):$(TAG)

external: buildExternalChecker pushExternalChecker

buildExternalChecker:
	docker build -t integrii/test-external-check:latest -f cmd/test-external-check/Dockerfile .

pushExternalChecker:
	docker push integrii/test-external-check:latest

podRestarts: buildPodRestartsCheck pushPodRestartsCheck

buildPodRestartsCheck:
	docker build -t quay.io/comcast/pod-restarts-check:1.0.0 -f cmd/podRestartsExternalCheck/Dockerfile .

pushPodRestartsCheck:
	docker push quay.io/comcast/pod-restarts-check:1.0.0