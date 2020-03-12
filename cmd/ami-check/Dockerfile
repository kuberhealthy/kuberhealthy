FROM golang:1.14.0 AS builder
ADD . /build
WORKDIR /build/cmd/ami-check
RUN CGO_ENABLED=0 go build -v

FROM scratch
COPY --from=builder /build/cmd/ami-check/ami-check /app/ami-check
ENTRYPOINT ["/app/ami-check"]
