FROM golang:alpine

ADD . /app

WORKDIR /app/cmd/car-checker

RUN go build -v

ENTRYPOINT /app/cmd/car-checker/car-checker
