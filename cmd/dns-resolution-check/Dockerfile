FROM golang AS builder
ADD . /build
WORKDIR /build/cmd/dns-resolution-check
ENV CGO_ENABLED=0
RUN go test -v
RUN go build -v

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/dns-resolution-check/dns-resolution-check /app/dns-resolution-check
ENTRYPOINT ["/app/dns-resolution-check"]
