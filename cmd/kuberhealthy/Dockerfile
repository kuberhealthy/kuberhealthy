FROM golang:latest as builder
LABEL LOCATION="git@github.com:Comcast/kuberhealthy.git"
LABEL DESCRIPTION="Kuberhealthy - Check and expose kubernetes cluster health in detail."
RUN mkdir /kuberhealthy

WORKDIR /go/src/github.com/Comcast/kuberhealthy/
COPY go.* ./
RUN go mod download
COPY ./ ./
WORKDIR /go/src/github.com/Comcast/kuberhealthy/cmd/kuberhealthy
ENV CGO_ENABLED=0
# RUN go version
#RUN go test -v -short -- --debug --forceMaster
RUN go build -v -o kuberhealthy
RUN mv kuberhealthy /kuberhealthy/kuberhealthy
RUN groupadd -g 999 kuberhealthy && useradd -r -u 999 -g kuberhealthy kuberhealthy

FROM scratch
WORKDIR /app
COPY --from=builder /kuberhealthy/ /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
USER kuberhealthy
EXPOSE 80
ENTRYPOINT /app/kuberhealthy
