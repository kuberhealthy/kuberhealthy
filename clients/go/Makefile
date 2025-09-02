BINARY := example-check
IMAGE ?= example-check:latest

build:
	go build -o $(BINARY) .

docker-build:
	docker build -t $(IMAGE) -f Dockerfile ../..

docker-push:
	docker push $(IMAGE)

.PHONY: build docker-build docker-push
