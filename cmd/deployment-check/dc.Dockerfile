ARG GO_VERSION=1.17
FROM docker-hub-remote.dr.corp.adobe.com/golang:${GO_VERSION}-alpine AS builder
WORKDIR /build
COPY go.* /build/
RUN go mod download

COPY . /build
WORKDIR /build/cmd/deployment-check
ENV CGO_ENABLED=0
RUN go build -v
FROM scratch
COPY --from=builder /etc/passwd /etc/passwd
USER user
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cmd/deployment-check/deployment-check /app/deployment-check
ENTRYPOINT ["/app/deployment-check"]