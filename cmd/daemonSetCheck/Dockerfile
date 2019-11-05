FROM golang:1.13-alpine AS builder
RUN apk upgrade
ADD . /build
WORKDIR /build/cmd/daemonSetCheck
ENV CGO_ENABLED=0
RUN go test -v
RUN go build -v

FROM golang:1.13-alpine
COPY --from=builder /build/cmd/daemonSetCheck/daemonSetCheck /app/daemonSetCheck
ENTRYPOINT ["/app/daemonSetCheck"]

