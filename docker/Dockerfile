FROM alpine:latest

MAINTAINER Sergio Correia <sergio@correia.cc>

RUN addgroup -g 1000 opbot
RUN adduser -u 1000 -S -G opbot -s /bin/bash -h /home/opbot opbot

RUN apk update && \
    apk add --no-cache bash \
    coreutils \
    sudo \
    alpine-sdk \
    wget \
    go \
    indent

ENV PROJECT=op-bot \
    GOPATH=/home/opbot/go \
    SETUP_DIR=/setup \
    ASTYLE_VER=3.0.1

# The setup step compiles whatever is needed.
COPY setup/ ${SETUP_DIR}/
RUN bash ${SETUP_DIR}/install.sh

ADD scripts/ /op-bot/
ENTRYPOINT ["/op-bot/start"]
