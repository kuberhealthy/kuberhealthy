FROM golang as builder
LABEL LOCATION="git@github.com:Comcast/kuberhealthy.git"
LABEL DESCRIPTION="Kuberhealthy - Check and expose kubernetes cluster health in detail."
ADD ./ /go/src/github.com/Comcast/kuberhealthy/
WORKDIR /go/src/github.com/Comcast/kuberhealthy/cmd/kuberhealthy
ENV GO111MODULE=on
ENV CGO_ENABLED=0
RUN go version
#RUN go test -v -short -- --debug --forceMaster
RUN go build -v -o kuberhealthy
RUN mkdir /kuberhealthy
RUN mv kuberhealthy /kuberhealthy/kuberhealthy

FROM golang
RUN apt-get update
RUN apt-get upgrade -y
RUN apt-get remove mercurial -y
RUN mkdir /app
WORKDIR /app
COPY --from=builder /kuberhealthy/ /app

RUN groupadd -g 999 kuberhealthy && useradd -r -u 999 -g kuberhealthy kuberhealthy
USER kuberhealthy
EXPOSE 80
ENTRYPOINT /app/kuberhealthy
