FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/kiam-check
RUN CGO_ENABLED=0 go build -v

FROM golang:1.13.3-alpine
COPY --from=builder /build/cmd/kiam-check/kiam-check /app/kiam-check
ENTRYPOINT ["/app/kiam-check"]
