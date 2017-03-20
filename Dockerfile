FROM alpine:3.5
MAINTAINER Oliver Soell <oliver@soell.net>

RUN apk add --update ca-certificates openssl && \
    rm -rf /var/cache/apk/*
RUN adduser -u 10001 -D -h /app app

USER app
CMD ["/opt/bin/rumours"]

COPY rumours /opt/bin/rumours

