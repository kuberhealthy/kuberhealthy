FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/podRestartsCheck
RUN go build -v

FROM golang:1.13
COPY --from=builder /build/cmd/podRestartsCheck/podRestartsCheck /app/podRestartsCheck
ENTRYPOINT ["/app/podRestartsCheck"]
