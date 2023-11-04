# This Makefile is imported from each check binary to make the management of all the different Dockerfiles easier.

IMAGE ?= kuberhealthy
TAG ?= unstable

build:
	docker build -f Dockerfile --progress=plain -t ${IMAGE}:${TAG} ../..

push:
	# Remove dangling builder instance that can show up when a build fails
	docker buildx rm $(BUILDER) || true
	docker buildx create --platform linux/amd64,linux/arm64 --name=$(BUILDER)
	docker buildx use $(BUILDER)
	docker buildx build --progress=plain --platform=linux/amd64,linux/arm64 --push -t ${IMAGE}:${TAG} -f Dockerfile ../../
	docker buildx prune --force
	docker buildx stop $(BUILDER)
	docker buildx rm $(BUILDER)
