FROM golang:1.13 AS builder
RUN groupadd -g 999 docker && \
    useradd -r -u 999 -g docker docker
USER docker
COPY [--chown=docker:docker] . /build
WORKDIR /build/cmd/http-content-check
ENV CGO_ENABLED=0
RUN go build -v
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/http-content-check/http-content-check /app/http-content-check
ENTRYPOINT ["/app/http-content-check"]
