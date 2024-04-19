FROM golang:1.22.2 as builder
LABEL LOCATION="git@github.com:kuberhealthy/kuberhealthy.git"
LABEL DESCRIPTION="Kuberhealthy - Check and expose kubernetes cluster health in detail."
COPY . /build
WORKDIR /build/cmd/kuberhealthy
ENV CGO_ENABLED=0
RUN go version
RUN go test -v
RUN (cd ../../pkg/metrics && go test -v)
RUN mkdir /app
RUN groupadd -g 999 user && \
    useradd -r -u 999 -g user user
RUN go build -v -o /app/kuberhealthy

FROM scratch
WORKDIR /app
COPY --from=builder /app /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
ENTRYPOINT ["/app/kuberhealthy"]
