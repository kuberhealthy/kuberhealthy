FROM golang:1.13 AS builder
ADD . /go/src/app
WORKDIR /go/src/app/cmd/deployment-check
RUN go build -o app -v

FROM golang:alpine
COPY --from=builder /go/src/app/cmd/deployment-check /app/app
ENTRYPOINT /app/app
