IMAGE ?= bash-check
TAG ?= latest

build:
	docker build -t $(IMAGE):$(TAG) .

push:
	docker push $(IMAGE):$(TAG)
