# This Makefile is imported from each check binary to make the management of all the different Dockerfiles easier.

IMAGE ?= kuberhealthy
TAG ?= unstable

# this is used as an include from the packages within the cmd/ directory
build:
	docker build -f Dockerfile --progress=plain -t ${IMAGE}:${TAG} ../..

# this is used as an include from the packages within the cmd/ directory
push:
	# Remove dangling builder instance that can show up when a build fails
	docker buildx rm $(BUILDER) || true
	docker buildx create --platform linux/amd64,linux/arm64 --name=$(BUILDER)
	docker buildx use $(BUILDER)
	docker buildx build --progress=plain --platform=linux/amd64,linux/arm64 --push -t ${IMAGE}:${TAG} -f Dockerfile ../../
	docker buildx prune --force
	docker buildx stop $(BUILDER)
	docker buildx rm $(BUILDER)


# https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/generating-clientset.md
generate:
	if [[ -z `which client-gen` ]]; then go install k8s.io/code-generator/cmd/client-gen@latest; fi
	rm -rf ./pkg/clients/generated
	client-gen -n khcheckClient --input-base "github.com/kuberhealthy/kuberhealthy/v2/pkg" --input apis/khcheck/v1 --output-pkg github.com/kuberhealthy/kuberhealthy/v2/pkg/clients/generated --output-dir ./pkg/clients/generated
	client-gen -n khstateClient --input-base "github.com/kuberhealthy/kuberhealthy/v2/pkg" --input apis/khstate/v1 --output-pkg github.com/kuberhealthy/kuberhealthy/v2/pkg/clients/generated --output-dir ./pkg/clients/generated
	client-gen -n khjobClient --input-base "github.com/kuberhealthy/kuberhealthy/v2/pkg" --input apis/khjob/v1 --output-pkg github.com/kuberhealthy/kuberhealthy/v2/pkg/clients/generated --output-dir ./pkg/clients/generated
