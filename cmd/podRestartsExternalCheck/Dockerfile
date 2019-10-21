FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/podRestartsExternalCheck
RUN go build -v

FROM golang:1.13
COPY --from=builder /build/cmd/podRestartsExternalCheck/podRestartsExternalCheck /app/podRestartsExternalCheck
ENTRYPOINT ["/app/podRestartsExternalCheck"]
