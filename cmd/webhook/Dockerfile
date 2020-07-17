FROM golang:1.14 AS builder
WORKDIR /build
COPY go.* /build/
RUN go mod download

COPY . /build
WORKDIR /build/cmd/webhook
ENV CGO_ENABLED=0
RUN go build -v
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/webhook/webhook /app/webhook
ENTRYPOINT ["/app/webhook"]
