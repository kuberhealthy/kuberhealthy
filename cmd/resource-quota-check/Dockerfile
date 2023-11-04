FROM golang:1.20 AS builder
WORKDIR /build
COPY go.* /build/
RUN go mod download

COPY . /build
WORKDIR /build/cmd/resource-quota-check
ENV CGO_ENABLED=0
RUN go build -v
RUN groupadd -g 999 user && \
    useradd -r -u 999 -g user user
FROM scratch
COPY --from=builder /etc/passwd /etc/passwd
USER user
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/resource-quota-check/resource-quota-check /app/resource-quota-check
ENTRYPOINT ["/app/resource-quota-check"]
