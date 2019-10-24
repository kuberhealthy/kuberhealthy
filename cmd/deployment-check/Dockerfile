FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/deployment-check
RUN CGO_ENABLED=0 go build -v

FROM golang:1.13.3-alpine
COPY --from=builder /build/cmd/deployment-check/deployment-check /app/deployment-check
ENTRYPOINT ["/app/deployment-check"]
