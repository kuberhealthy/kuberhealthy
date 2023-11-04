FROM golang:1.20 AS builder
RUN groupadd -g 999 user && \
    useradd -r -u 999 -g user user
COPY --chown=user:user . /build
WORKDIR /build/cmd/network-connection-check
ENV CGO_ENABLED=0
RUN go build -v
FROM scratch
COPY --from=builder /etc/passwd /etc/passwd
USER 65534
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/network-connection-check/network-connection-check /app/network-connection-check
ENTRYPOINT ["/app/network-connection-check"]
