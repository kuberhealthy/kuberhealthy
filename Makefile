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

daemonset: buildDaemonsetCheck pushDaemonsetCheck

buildDaemonsetCheck:
	docker build -t docker-proto.repo.theplatform.com/kh-check-daemonset:1.0.0 -f cmd/daemonSetExternalCheck/Dockerfile .

pushDaemonsetCheck:
	docker push docker-proto.repo.theplatform.com/kh-check-daemonset:1.0.0
