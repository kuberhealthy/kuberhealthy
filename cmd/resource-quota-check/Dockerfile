FROM golang AS builder
ADD . /build
WORKDIR /build/cmd/resource-quota-check
RUN CGO_ENABLED=0 go build -v

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/resource-quota-check/resource-quota-check /app/resource-quota-check
ENTRYPOINT ["/app/resource-quota-check"]
