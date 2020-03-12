FROM golang:1.14.0

ADD . /build

WORKDIR /build/cmd/http-content-check

RUN go build -v

ENTRYPOINT /build/cmd/http-content-check/http-content-check
