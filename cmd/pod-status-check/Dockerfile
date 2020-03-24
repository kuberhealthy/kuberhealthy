FROM golang AS builder
ADD . /build
WORKDIR /build/cmd/pod-status-check
ENV CGO_ENABLED=0
RUN go build -v

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/pod-status-check/pod-status-check /app/pod-status-check
ENTRYPOINT ["/app/pod-status-check"]
