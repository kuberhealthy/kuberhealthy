FROM golang:1.13 AS builder
ADD . /build
WORKDIR /build/cmd/testExternalCheck
RUN go build -v

FROM golang:1.13
COPY --from=builder /build/cmd/testExternalCheck/testExternalCheck /app/testExternalCheck
ENTRYPOINT ["/app/testExternalCheck"]

