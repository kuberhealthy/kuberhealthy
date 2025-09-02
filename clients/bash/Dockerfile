FROM alpine:3.20
RUN apk add --no-cache bash curl
COPY check.sh /check.sh
ENTRYPOINT ["/check.sh"]
