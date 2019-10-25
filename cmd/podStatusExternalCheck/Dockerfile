FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/podStatusCheck
RUN go build -v

FROM golang:1.13
COPY --from=builder /build/cmd/podStatusCheck/podStatusCheck /app/podStatusCheck
ENTRYPOINT ["/app/podStatusCheck"]

