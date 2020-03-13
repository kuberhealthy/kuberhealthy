FROM golang AS builder
RUN apk upgrade
ADD . /build
WORKDIR /build/cmd/pod-restarts-check
ENV CGO_ENABLED=0
RUN go build -v

FROM scratch
FROM golang:1.13-alpine
COPY --from=builder /build/cmd/pod-restarts-check/pod-restarts-check /app/pod-restarts-check
ENTRYPOINT ["/app/pod-restarts-check"]
