FROM golang:1.13 AS builder
RUN groupadd -g 61000 docker
RUN useradd -g 61000 -l -m -s /bin/false -u 61000 docker
USER docker
COPY . /build
WORKDIR /build/cmd/http-content-check
ENV CGO_ENABLED=0
RUN go build -v
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/http-content-check/http-content-check /app/http-content-check
ENTRYPOINT ["/app/http-content-check"]
