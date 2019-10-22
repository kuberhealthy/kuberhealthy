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

deploymentCheck: buildDeploymentCheck pushDeploymentCheck

buildDeploymentCheck:
	docker build -t quay.io/comcast/deployment-check:1.0.0 -f cmd/deployment-check/Dockerfile .

pushDeploymentCheck:
	docker push quay.io/comcast/deployment-check:1.0.0