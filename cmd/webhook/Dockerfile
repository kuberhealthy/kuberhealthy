FROM golang:1.14 AS builder
WORKDIR /build
COPY go.* /build/
RUN go mod download

COPY . /build
WORKDIR /build/cmd/webhook
ENV CGO_ENABLED=0
RUN go build -v
RUN groupadd -g 999 user && \
    useradd -r -u 999 -g user user
FROM scratch
COPY --from=builder /etc/passwd /etc/passwd
USER user
COPY --from=builder /etc/webhook/certs/ /etc/webhook/certs/
COPY --from=builder /build/cmd/webhook/webhook /app/webhook
ENTRYPOINT ["/app/webhook"]
