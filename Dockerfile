FROM golang as builder
LABEL LOCATION="git@github.com:Comcast/kuberhealthy.git"
LABEL DESCRIPTION="Kuberhealthy - Check and expose kubernetes cluster health in detail."
RUN mkdir -p /go/src/github.com/Comcast/kuberhealthy/pkg/kubeClient
ADD ./ /go/src/github.com/Comcast/kuberhealthy/pkg/
WORKDIR /go/src/github.com/Comcast/kuberhealthy/pkg/cmd/kuberhealthy
RUN go get -v
RUN go build -v -o kuberhealthy
RUN mkdir /kuberhealthy
RUN cp kuberhealthy /kuberhealthy/kuberhealthy

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
