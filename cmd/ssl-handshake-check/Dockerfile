FROM golang:1.20 AS builder
RUN groupadd -g 999 user && \
    useradd -r -u 999 -g user user
COPY --chown=user:user . /build
WORKDIR /build/cmd/ssl-handshake-check
ENV CGO_ENABLED=0
RUN go build -v
FROM scratch
COPY --from=builder /etc/passwd /etc/passwd
USER user
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/ssl-handshake-check/ssl-handshake-check /app/ssl-handshake-check
ENTRYPOINT ["/app/ssl-handshake-check"]
