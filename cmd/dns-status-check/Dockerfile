FROM golang AS builder
RUN apk upgrade
ADD . /build
WORKDIR /build/cmd/dns-status-check
ENV CGO_ENABLED=0
RUN go test -v
RUN go build -v

FROM scratch
COPY --from=builder /build/cmd/dns-status-check/dns-status-check /app/dns-status-check
ENTRYPOINT ["/app/dns-status-check"]
