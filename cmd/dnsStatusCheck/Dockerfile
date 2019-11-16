FROM golang:1.13-alpine AS builder
RUN apk upgrade
ADD . /build
WORKDIR /build/cmd/dnsStatusCheck
ENV CGO_ENABLED=0
RUN go test -v
RUN go build -v

FROM golang:1.13-alpine
COPY --from=builder /build/cmd/dnsStatusCheck/dnsStatusCheck /app/dnsStatusCheck
ENTRYPOINT ["/app/dnsStatusCheck"]
