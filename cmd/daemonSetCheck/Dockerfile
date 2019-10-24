FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/daemonSetCheck
RUN go build -v

FROM golang:1.13
COPY --from=builder /build/cmd/daemonSetCheck/daemonSetCheck /app/daemonSetCheck
ENTRYPOINT ["/app/daemonSetCheck"]

