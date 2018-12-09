FROM alpine:latest

MAINTAINER Marco Paganini <paganini@paganini.net> and Sergio Correia <sergio@correia.cc>

ARG OPBOT_PROJECT="op-bot"
ARG OPBOT_USER="opbot"
ARG OPBOT_HOME_DIR="/home/${OPBOT_USER}"
ARG GITHUB_URL="https://github.com/OsProgramadores/${OPBOT_PROJECT}.git"
ARG GOPATH="/tmp/go"
ARG OPBOT_GIT_ROOT="${GOPATH}/src/${OPBOT_PROJECT}"
ARG OPBOT_UID=1000

RUN addgroup -g ${OPBOT_UID} ${OPBOT_USER} && \
    adduser -u ${OPBOT_UID} -S -G ${OPBOT_USER} -s /bin/bash -h ${OPBOT_HOME_DIR} ${OPBOT_USER} && \
    apk update && \
    apk add --no-cache bash coreutils libc-dev wget git go && \
    mkdir -p "${OPBOT_GIT_ROOT}" && \
    git clone "${GITHUB_URL}" "${OPBOT_GIT_ROOT}" && \
    cd "${OPBOT_GIT_ROOT}" && \
    go get -v && \
    go build && \
    mv "${OPBOT_PROJECT}" /usr/bin && \
    cd /tmp && \
    rm -rf "${GOPATH}" && \
    apk add ca-certificates && \
    apk del coreutils libc-dev wget git go && \
    rm -rf /var/cache/apk/*

ENTRYPOINT ["/bin/su", "-", "opbot", "-c", "'/usr/bin/op-bot'"]

