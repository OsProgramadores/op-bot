FROM alpine:latest

MAINTAINER Marco Paganini <paganini@paganini.net> and Sergio Correia <sergio@correia.cc>

ARG OPBOT_PROJECT="op-bot"
ARG OPBOT_USER="opbot"
ARG OPBOT_HOME_DIR="/home/${OPBOT_USER}"
ARG SETUP_DIR="/setup"
ARG SETUP_SCRIPT="https://raw.githubusercontent.com/OsProgramadores/op-bot/master/docker/setup/install.sh"
ARG ASTYLE_VER="3.0.1"
ARG GITHUB_URL="https://github.com/OsProgramadores/${OPBOT_PROJECT}.git"
ARG GOPATH="/tmp/go"
ARG OPBOT_GIT_ROOT="${GOPATH}/src/${OPBOT_PROJECT}"

ADD setup/install.sh /tmp

# This invalidates the cache for all run commands
# from this point on. Without this, Docker build
# won't re-run the RUN commands on newer builds.
ADD cachebuster /tmp

RUN addgroup -g 1000 ${OPBOT_USER} && \
    adduser -u 1000 -S -G ${OPBOT_USER} -s /bin/bash -h ${OPBOT_HOME_DIR} ${OPBOT_USER} && \
    apk update && \
    apk add --no-cache bash tar coreutils wget git g++ make libc-dev go indent && \
    mkdir -p "${OPBOT_GIT_ROOT}" && \
    git clone "${GITHUB_URL}" "${OPBOT_GIT_ROOT}" && \
    cd "${OPBOT_GIT_ROOT}" && \
    go get -v && \
    go build && \
    mv "${OPBOT_PROJECT}" /usr/bin && \
    cd /tmp && \
    rm -rf "${GOPATH}" && \
    mkdir -p "${SETUP_DIR}" && \
    cd "${SETUP_DIR}" && \
    bash /tmp/install.sh && \
    rm -rf /tmp/install.sh "${SETUP_DIR}" && \
    apk del tar coreutils wget git g++ make libc-dev go && \
    apk add ca-certificates libstdc++ && \
    rm -rf /var/cache/apk/*

ENTRYPOINT ["/bin/su", "-", "opbot", "-c", "'/usr/bin/op-bot'"]

