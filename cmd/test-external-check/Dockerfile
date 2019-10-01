FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/test-external-check
RUN go build -v

FROM golang:1.13
COPY --from=builder /build/cmd/test-external-check/test-external-check /app/test-external-check
ENTRYPOINT ["/app/test-external-check"]

