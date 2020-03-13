FROM golang AS builder
ADD . /build
WORKDIR /build/cmd/http-check
RUN CGO_ENABLED=0 go build -v

FROM scratch
COPY --from=builder /build/cmd/http-check/http-check /app/http-check
ENTRYPOINT ["/app/http-check"]
