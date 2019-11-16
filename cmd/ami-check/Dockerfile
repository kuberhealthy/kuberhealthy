FROM golang AS builder
ADD . /build
WORKDIR /build/cmd/ami-check
RUN CGO_ENABLED=0 go build -v -o ami-check

FROM golang:alpine
COPY --from=builder /build/cmd/ami-check/ami-check /app/ami-check
ENTRYPOINT ["/app/ami-check"]
