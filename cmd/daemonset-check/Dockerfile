FROM golang:1.13-alpine AS builder
RUN apk upgrade
ADD . /build
WORKDIR /build/cmd/daemonset-check
ENV CGO_ENABLED=0
RUN go test -v
RUN go build -v

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/daemonset-check/daemonset-check /app/daemonset-check
ENTRYPOINT ["/app/daemonset-check"]
