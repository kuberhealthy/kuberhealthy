FROM golang:1.14.0 AS builder
ADD . /build
WORKDIR /build/cmd/test-external-check
RUN go build -v

FROM scrapinghub/scrapinghub-stack-hworker
COPY --from=builder /build/cmd/test-external-check/test-external-check /app/test-external-check
ENTRYPOINT ["/app/test-external-check"]

