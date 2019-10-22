FROM golang:alpine AS builder
ADD . /go/src/app
WORKDIR /go/src/app/cmd/deployment-check
RUN GOOS=linux GOARCH=amd64 go build -o app -v

FROM scratch
COPY --from=builder /go/src/app/cmd/deployment-check /app/app
ENTRYPOINT /app/app
