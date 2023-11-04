FROM golang:1.20 AS builder
COPY . /build
RUN ls -alR /build
WORKDIR /build/cmd/http-content-check
RUN CGO_ENABLED=0 go build -v
RUN groupadd -g 999 user && useradd -r -u 999 -g user user


FROM scratch
COPY --from=builder /etc/passwd /etc/passwd
USER user
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/http-content-check/http-content-check /app/http-content-check
ENTRYPOINT ["/app/http-content-check"]
