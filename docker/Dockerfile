FROM alpine:latest

MAINTAINER Sergio Correia <sergio@correia.cc>

RUN apk update && \
    apk add --no-cache bash \
    coreutils \
    alpine-sdk \
    wget \
    go \
    indent && \
    mkdir -p /root/go

ENV PROJECT=op-bot \
    GOPATH=/go \
    SETUP_DIR=/setup \
    ASTYLE_VER=3.0.1

# The setup step compiles whatever is needed.
COPY setup/ ${SETUP_DIR}/
RUN bash ${SETUP_DIR}/install.sh

ADD scripts/ /op-bot/
ENTRYPOINT ["/op-bot/start"]
