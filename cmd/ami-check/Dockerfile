FROM golang AS builder
ADD . /build
WORKDIR /build/ami-check
RUN CGO_ENABLED=0 go build -v

FROM scratch
COPY --from=builder /build/ami-check/ami-check /app/ami-check
ENTRYPOINT ["/app/ami-check"]
