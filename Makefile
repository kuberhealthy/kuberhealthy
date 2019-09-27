<<<<<<< HEAD
# IMAGE="quay.io/comcast/kuberhealthy"
# TAG="2.0.0alpha4"

# build:
# 	docker build -t $(IMAGE):$(TAG) .

# push:
# 	docker push $(IMAGE):$(TAG)

# buildExternalChecker:
# 	docker build -t testexternalcheck:latest -f cmd/testExternalCheck/Dockerfile .

VERSION="2.0.0alphajonny1"
IMAGE="kuberhealthy:$(VERSION)"
IMAGE_HOST="docker-proto.repo.theplatform.com"
=======
IMAGE="quay.io/comcast/kuberhealthy"
TAG="2.0.0alpha5"
>>>>>>> 61cabfdc9eaf0dc3bf51baf874778c64cfbf40d3

build:
	docker build -t $(IMAGE) .

tag:
	docker tag $(IMAGE) $(IMAGE_HOST)/$(IMAGE)

push: tag
	docker push $(IMAGE_HOST)/$(IMAGE)
buildExternalChecker:
	docker build -t quay.io/comcast/testexternalcheck:latest -f cmd/testExternalCheck/Dockerfile .

pushExternalChecker:
	docker push quay.io/comcast/testexternalcheck:latest
