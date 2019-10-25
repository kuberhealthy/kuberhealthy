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

deploymentCheck: buildDeploymentCheck pushDeploymentCheck

buildDeploymentCheck:
	docker build -t quay.io/comcast/deployment-check:1.0.0alpha -f cmd/deployment-check/Dockerfile .

pushDeploymentCheck:
	docker push quay.io/comcast/deployment-check:1.0.0alpha

daemonset: buildDaemonsetCheck pushDaemonsetCheck

buildDaemonsetCheck:
	docker build -t quay.io/comcast/kh-daemonset-check:1.0.0 -f cmd/daemonSetCheck/Dockerfile .

pushDaemonsetCheck:
	docker push quay.io/comcast/kh-daemonset-check:1.0.0

podRestarts: buildPodRestartsCheck pushPodRestartsCheck

buildPodRestartsCheck:
	docker build -t quay.io/comcast/pod-restarts-check:1.0.0 -f cmd/podRestartsCheck/Dockerfile .

pushPodRestartsCheck:
	docker push quay.io/comcast/pod-restarts-check:1.0.0

dnsStatus: buildDNSStatusCheck pushDNSStatusCheck

buildDNSStatusCheck:
	docker build -t quay.io/comcast/dns-status-check:1.0.0 -f cmd/dnsStatusCheck/Dockerfile .

pushDNSStatusCheck:
	docker push quay.io/comcast/dns-status-check:1.0.0