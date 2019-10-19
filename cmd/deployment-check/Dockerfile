FROM golang:1.13 AS builder
RUN apt-get upgrade -y && apt-get install git -y
ADD . /go/src/app
WORKDIR /go/src/app
RUN go get -v
RUN go build -o app
RUN mkdir /app
RUN mv app /app/app

FROM golang:1.13
COPY --from=builder /app/app /app/app
ENTRYPOINT /app/app
