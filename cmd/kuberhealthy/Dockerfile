FROM golang:1.13 as builder
LABEL LOCATION="git@github.com:Comcast/kuberhealthy.git"
LABEL DESCRIPTION="Kuberhealthy - Check and expose kubernetes cluster health in detail."
RUN curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b $GOPATH/bin 2.0.0
ADD ./ /go/src/github.com/Comcast/kuberhealthy/
WORKDIR /go/src/github.com/Comcast/kuberhealthy/cmd/kuberhealthy
ENV CGO_ENABLED=0
RUN go version
RUN gosec ./...
#RUN go test -v -short -- --debug --forceMaster
RUN go build -v -o kuberhealthy
RUN mkdir /kuberhealthy
RUN mv kuberhealthy /kuberhealthy/kuberhealthy

FROM golang:1.13
RUN apt-get update && apt-get upgrade -y && apt-get remove mercurial -y && rm -rf /var/lib/apt/lists/*
RUN mkdir /app
WORKDIR /app
COPY --from=builder /kuberhealthy/ /app

RUN groupadd -g 999 kuberhealthy && useradd -r -u 999 -g kuberhealthy kuberhealthy
USER kuberhealthy
EXPOSE 80
ENTRYPOINT /app/kuberhealthy
