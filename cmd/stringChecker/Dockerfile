FROM golang:alpine

ADD . /build

WORKDIR /build/cmd/stringChecker

RUN go build -v

ENTRYPOINT /build/cmd/stringChecker/stringChecker
