#!/usr/bin/env bash

go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1
go get github.com/brancz/gojsontoyaml

GOPATH=$(go env GOPATH)

go run -v ./scripts/generate-crds.go --controller-gen=${GOPATH}/bin/controller-gen --gojsontoyaml=${GOPATH}/bin/gojsontoyaml

echo $(pwd)

## Edit the khcheck and khjob crds files to have `preserveUnknownFields: false`
awk '{print} sub(/scope: Namespaced/,"preserveUnknownFields: false")' ./scripts/generated/comcast.github.io_khchecks.yaml > tmpfile.yaml  && mv tmpfile.yaml ./scripts/generated/comcast.github.io_khchecks.yaml
awk '{print} sub(/scope: Namespaced/,"preserveUnknownFields: false")' ./scripts/generated/comcast.github.io_khjobs.yaml > tmpfile.yaml  && mv tmpfile.yaml ./scripts/generated/comcast.github.io_khjobs.yaml
