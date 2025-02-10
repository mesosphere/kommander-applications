FROM alpine:3.21.2

LABEL org.opencontainers.image.source=https://github.com/alpinelinux/docker-alpine

RUN apk add --update git && \
    apk add --update git-daemon \
    rm -rf /var/cache/apk/*

COPY --chown=1000:1000 data /data

CMD ["git", "daemon", "--verbose", "--port=12345","--export-all", "/data"]
