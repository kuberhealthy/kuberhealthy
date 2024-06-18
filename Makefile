# This Makefile is imported from each check binary to make the management of all the different Dockerfiles easier. You should not run this Makefile directly.

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
# TODO - What other types of generation do we need here?  Probably deepcopy and others?
generate:
	cd hack && ./update-codegen.sh


