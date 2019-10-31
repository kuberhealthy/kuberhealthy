FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/dnsStatusCheck
RUN go build -v

FROM golang:1.13
COPY --from=builder /build/cmd/dnsStatusCheck/dnsStatusCheck /app/dnsStatusCheck
ENTRYPOINT ["/app/dnsStatusCheck"]

