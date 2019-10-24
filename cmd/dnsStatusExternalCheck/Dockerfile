FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/dnsStatusExternalCheck
RUN go build -v

FROM golang:1.13
COPY --from=builder /build/cmd/dnsStatusExternalCheck/dnsStatusExternalCheck /app/dnsStatusExternalCheck
ENTRYPOINT ["/app/dnsStatusExternalCheck"]

