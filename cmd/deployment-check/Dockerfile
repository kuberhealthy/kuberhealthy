FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/deployment-check
ENV GOOS=linux GOARCH=amd64 CGO_ENABLED=0
RUN go build -v

FROM golang:1.13.3-alpine
RUN mkdir /app
COPY --from=builder /build/cmd/deployment-check/deployment-check /app
ENTRYPOINT /app/deployment-check