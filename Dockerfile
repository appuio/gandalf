FROM docker.io/library/debian:12 as runtime

RUN \
  apt update && \
  apt install \
    bash \
    coreutils

COPY docker-entrypoint.sh /usr/bin/
COPY gandalf /usr/bin/

ENTRYPOINT ["bash", "/usr/bin/docker-entrypoint.sh"]

USER 65536:0
