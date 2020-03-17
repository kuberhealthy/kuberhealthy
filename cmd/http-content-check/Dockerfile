FROM golang AS builder

ADD . /build

WORKDIR /build/cmd/http-content-check

ENV CGO_ENABLED=0
RUN go build -v

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/http-content-check/http-content-check /app/http-content-check

ENTRYPOINT /app/http-content-check
