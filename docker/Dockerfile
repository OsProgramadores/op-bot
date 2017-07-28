FROM alpine:latest

MAINTAINER Sergio Correia <sergio@correia.cc>

RUN apk update && \
    apk add --no-cache bash \
    coreutils \
    sudo \
    alpine-sdk \
    wget \
    go \
    indent

ARG OPBOT_PROJECT="op-bot"
ARG OPBOT_USER="opbot"
ARG OPBOT_HOME_DIR="/home/${OPBOT_USER}"
ARG SETUP_DIR="/setup"
ARG ASTYLE_VER="3.0.1"
ARG GITHUB_URL="https://github.com/OsProgramadores/${OPBOT_PROJECT}.git"
ARG GOPATH="/tmp/go"
ARG OPBOT_GIT_ROOT="${GOPATH}/src/${OPBOT_PROJECT}"

RUN addgroup -g 1000 ${OPBOT_USER}
RUN adduser -u 1000 -S -G ${OPBOT_USER} -s /bin/bash -h ${OPBOT_HOME_DIR} ${OPBOT_USER}

# Configure the required GOPATH structure and compile op-bot.
# Move the binary to /tmp and remove the $GOPATH structure.
RUN mkdir -p "${OPBOT_GIT_ROOT}" && \
    git clone "${GITHUB_URL}" "${OPBOT_GIT_ROOT}" && \
    cd "${OPBOT_GIT_ROOT}" && \
    go get -v && \
    go build && \
    mv "${OPBOT_PROJECT}" /tmp && \
    cd /tmp && \
    rm -rf "${GOPATH}"

# The setup step compiles all dependencies.
COPY setup/ "${SETUP_DIR}/"
RUN bash "${SETUP_DIR}/install.sh"

ENTRYPOINT ["/usr/bin/sudo", "-u", "opbot", "/tmp/op-bot"]
