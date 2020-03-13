FROM golang AS builder
RUN apk upgrade
ADD . /build
WORKDIR /build/cmd/daemon-set-check
ENV CGO_ENABLED=0
RUN go test -v
RUN go build -v

FROM scratch
COPY --from=builder /build/cmd/daemon-set-check/daemon-set-check /app/daemon-set-check
ENTRYPOINT ["/app/daemon-set-check"]

