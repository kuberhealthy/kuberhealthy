IMAGE ?= kuberhealthy-rust-example:latest

build:
	cargo build --release

docker-build: build
	docker build -t $(IMAGE) .

push:
	docker push $(IMAGE)
