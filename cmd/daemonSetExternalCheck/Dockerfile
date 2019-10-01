FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/daemonSetExternalCheck
RUN go build -v

FROM golang:1.13
COPY --from=builder /build/cmd/daemonSetExternalCheck/daemonSetExternalCheck /app/daemonSetExternalCheck
ENTRYPOINT ["/app/daemonSetExternalCheck"]

