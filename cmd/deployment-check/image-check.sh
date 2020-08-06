#!/bin/bash

function docker_tag_exists() {
    curl --silent -f -lSL https://index.docker.io/v1/repositories/$1/tags/$2 > /dev/null
}

if docker_tag_exists $IMAGE $TAG; then
    echo "Image ${IMAGE}:${TAG} exists. Skipping build and push."
else 
    echo "Image ${IMAGE}:${TAG} does not exist. Building and pushing new image."
    make build && make push
fi