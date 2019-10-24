FROM golang:1.13 AS builder
ADD . /go/src/app
WORKDIR /go/src/app/cmd/deployment-check
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o app -v

FROM golang:1.13.3-alpine
COPY --from=builder /go/src/app/cmd/deployment-check /app/app
ENTRYPOINT /app/app
