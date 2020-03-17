FROM golang AS builder
ADD . /build
WORKDIR /build/cmd/test-external-check
RUN CGO_ENABLED=0 go build -v

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/test-external-check/test-external-check /app/test-external-check
ENTRYPOINT ["/app/test-external-check"]
