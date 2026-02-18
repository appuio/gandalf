FROM docker.io/library/debian:12 AS runtime

RUN \
  apt update && \
  apt install \
    bash \
    coreutils

COPY gandalf /usr/bin/

ENTRYPOINT ["gandalf"]

USER 65536:0
