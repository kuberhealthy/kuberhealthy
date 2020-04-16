FROM golang AS builder
ADD . /build
WORKDIR /build/cmd/pod-restarts-check
ENV CGO_ENABLED=0
RUN go build -v

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/pod-restarts-check/pod-restarts-check /app/pod-restarts-check
ENTRYPOINT ["/app/pod-restarts-check"]
