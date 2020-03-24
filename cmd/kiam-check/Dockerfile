FROM golang AS builder
ADD . /build
WORKDIR /build/cmd/kiam-check
RUN CGO_ENABLED=0 go build -v

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/kiam-check/kiam-check /app/kiam-check
ENTRYPOINT ["/app/kiam-check"]
