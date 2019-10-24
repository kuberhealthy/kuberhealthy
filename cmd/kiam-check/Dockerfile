FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/kiam-check
ENV GOOS=linux GOARCH=amd64 CGO_ENABLED=0
RUN go build -v

FROM golang:1.13.3
RUN mkdir /app
COPY --from=builder /build/cmd/kiam-check/kiam-check /app
ENTRYPOINT /app/kiam-check
