FROM golang:1.14.0 AS builder
ADD . /build
WORKDIR /build/cmd/deployment-check
RUN CGO_ENABLED=0 go build -v

FROM scratch
COPY --from=builder /build/cmd/deployment-check/deployment-check /app/deployment-check
ENTRYPOINT ["/app/deployment-check"]
