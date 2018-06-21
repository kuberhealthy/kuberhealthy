IMAGE="kuberheathy:$(VERSION)"
IMAGE_HOST="<YOUR_IMAGEHOST_HERE>"

build: clean
	#GOOS=linux go build -o app
	docker build -t $(IMAGE) .

clean:
	@-docker rmi -f $(shell docker images -q kuberhealthy)

tag:
	docker tag $(IMAGE) $(IMAGE_HOST)/$(IMAGE)

push: tag
	docker push $(IMAGE_HOST)/$(IMAGE)

localbuild:
	VERSION=0
	make build
