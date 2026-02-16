FROM docker.io/library/alpine:3.23 as runtime

RUN \
  apk add --update --no-cache \
    bash \
    coreutils

COPY docker-entrypoint.sh /usr/bin/
COPY gandalf /usr/bin/

ENTRYPOINT ["bash", "/usr/bin/docker-entrypoint.sh"]

USER 65536:0
