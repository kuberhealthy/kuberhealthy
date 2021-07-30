#!/usr/bin/env bash

go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.2
go get github.com/brancz/gojsontoyaml

GOPATH=$(go env GOPATH)

go run -v ./generate-crds.go --controller-gen=${GOPATH}/bin/controller-gen --gojsontoyaml=${GOPATH}/bin/gojsontoyaml

## Edit the khcheck and khjob crds files to have `preserveUnknownFields: false`
awk '{print} sub(/scope: Namespaced/,"preserveUnknownFields: false")' ./generated/comcast.github.io_khchecks.yaml > tmpfile.yaml  && mv tmpfile.yaml ./generated/comcast.github.io_khchecks.yaml
awk '{print} sub(/scope: Namespaced/,"preserveUnknownFields: false")' ./generated/comcast.github.io_khjobs.yaml > tmpfile.yaml  && mv tmpfile.yaml ./generated/comcast.github.io_khjobs.yaml
