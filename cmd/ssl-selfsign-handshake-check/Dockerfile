FROM golang:1.14 AS builder
RUN groupadd -g 999 user && \
    useradd -r -u 999 -g user user
COPY --chown=user:user . /build
WORKDIR /build/cmd/ssl-selfsign-handshake-check
ENV CGO_ENABLED=0
RUN go build -v
FROM scratch
COPY --from=builder /etc/passwd /etc/passwd
USER user
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/ssl-selfsign-handshake-check/ssl-selfsign-handshake-check /app/ssl-selfsign-handshake-check
ENTRYPOINT ["/app/ssl-selfsign-handshake-check"]