IMAGE="quay.io/comcast/kuberhealthy"
TAG="2.1.0"

build:
	docker build -t $(IMAGE):$(TAG) .

push:
	docker push $(IMAGE):$(TAG)

amiCheck: buildAMICheck pushAMICheck

buildAMICheck:
	docker build -t quay.io/comcast/ami-check:1.0.1 -f cmd/ami-check/Dockerfile .

pushAMICheck:
	docker push quay.io/comcast/ami-check:1.0.1

external: buildExternalChecker pushExternalChecker

buildExternalChecker:
	docker build -t quay.io/comcast/test-external-check:latest -f cmd/test-external-check/Dockerfile .

pushExternalChecker:
	docker push quay.io/comcast/test-external-check:latest

deploymentCheck: buildDeploymentCheck pushDeploymentCheck

buildDeploymentCheck:
	docker build -t quay.io/comcast/deployment-check:1.1.0 -f cmd/deployment-check/Dockerfile .

pushDeploymentCheck:
	docker push quay.io/comcast/deployment-check:1.1.0

daemonset: buildDaemonsetCheck pushDaemonsetCheck

buildDaemonsetCheck:
	docker build -t quay.io/comcast/kh-daemonset-check:2.0.0 -f cmd/daemonSetCheck/Dockerfile .

pushDaemonsetCheck:
	docker push quay.io/comcast/kh-daemonset-check:2.0.0

kiamCheck: buildKIAMCheck pushKIAMCheck

buildKIAMCheck:
	docker build -t quay.io/comcast/kiam-check:1.0.1 -f cmd/kiam-check/Dockerfile .

pushKIAMCheck:
	docker push quay.io/comcast/kiam-check:1.0.1

podRestarts: buildPodRestartsCheck pushPodRestartsCheck

buildPodRestartsCheck:
	docker build -t quay.io/comcast/pod-restarts-check:2.0.0 -f cmd/podRestartsCheck/Dockerfile .

pushPodRestartsCheck:
	docker push quay.io/comcast/pod-restarts-check:2.0.0

dnsStatus: buildDNSStatusCheck pushDNSStatusCheck

buildDNSStatusCheck:
	docker build -t quay.io/comcast/dns-status-check:1.0.0 -f cmd/dnsStatusCheck/Dockerfile .

pushDNSStatusCheck:
	docker push quay.io/comcast/dns-status-check:1.0.0

podStatus: buildPodStatusCheck pushPodStatusCheck

buildPodStatusCheck:
	docker build -t quay.io/comcast/pod-status-check:1.0.2 -f cmd/podStatusCheck/Dockerfile .

pushPodStatusCheck:
	docker push quay.io/comcast/pod-status-check:1.0.2

httpCheck: buildHTTPCheck pushHTTPCheck

buildHTTPCheck:
	docker build -t quay.io/comcast/http-check:1.0.0 -f cmd/http-check/Dockerfile .

pushHTTPCheck:
	docker push quay.io/comcast/http-check:1.0.0

checkReaper: buildCheckReaper pushCheckReaper

buildCheckReaper:
	docker build -t quay.io/comcast/check-reaper:1.0.0 -f cmd/check-reaper/Dockerfile .

pushCheckReaper:
	docker push quay.io/comcast/check-reaper:1.0.0
