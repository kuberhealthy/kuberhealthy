FROM golang

ADD . /build

WORKDIR /build/cmd/http-content-check

RUN go build -v

ENTRYPOINT /build/cmd/http-content-check/http-content-check
