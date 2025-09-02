IMAGE ?= kuberhealthy/java-example:latest

.PHONY: build
build:
	docker build -t $(IMAGE) .

.PHONY: push
push: build
	docker push $(IMAGE)
