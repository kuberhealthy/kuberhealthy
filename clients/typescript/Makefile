IMAGE ?= kuberhealthy-typescript-example:latest

build:
	npm install
	npm run build

docker-build:
	docker build -t $(IMAGE) .

docker-push:
	docker push $(IMAGE)
