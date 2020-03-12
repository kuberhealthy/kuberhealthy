FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/pod-status-check
RUN go build -v

FROM golang:1.13
COPY --from=builder /build/cmd/pod-status-check/pod-status-check /app/pod-status-check
ENTRYPOINT ["/app/pod-status-check"]
