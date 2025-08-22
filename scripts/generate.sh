#!/usr/bin/env bash

go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.2
go get github.com/brancz/gojsontoyaml

GOPATH=$(go env GOPATH)

go run -v ./generate-crds.go --controller-gen=${GOPATH}/bin/controller-gen --gojsontoyaml=${GOPATH}/bin/gojsontoyaml
