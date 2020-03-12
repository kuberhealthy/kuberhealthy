FROM golang:1.14 AS builder
ADD . /build
WORKDIR /build/cmd/resource-quota-check
RUN CGO_ENABLED=0 go build -v

# FROM golang:1.14.0-alpine
FROM scratch
COPY --from=builder /build/cmd/resource-quota-check/resource-quota-check /app/resource-quota-check
ENTRYPOINT ["/app/resource-quota-check"]
