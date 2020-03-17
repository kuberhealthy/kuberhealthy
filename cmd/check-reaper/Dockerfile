FROM golang AS builder
ADD . /build
WORKDIR /build/cmd/check-reaper
ENV CGO_ENABLED=0
RUN go build -v

FROM scratch
COPY --from=builder /build/cmd/check-reaper/check-reaper /app/check-reaper
ENTRYPOINT ["/app/check-reaper"]
