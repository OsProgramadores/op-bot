FROM alpine:latest

MAINTAINER Marco Paganini <paganini@paganini.net> and Sergio Correia <sergio@correia.cc>

ARG OPBOT_PROJECT="op-bot"
ARG OPBOT_USER="opbot"
ARG OPBOT_HOME_DIR="/home/${OPBOT_USER}"
ARG GITHUB_URL="https://github.com/OsProgramadores/${OPBOT_PROJECT}.git"
ARG GOPATH="/tmp/go"
ARG OPBOT_GIT_ROOT="${GOPATH}/src/${OPBOT_PROJECT}"
ENV UID 501

# This invalidates the cache for all run commands
# from this point on. Without this, Docker build
# won't re-run the RUN commands on newer builds.
ADD cachebuster /tmp

RUN addgroup -g ${UID} ${OPBOT_USER} && \
    adduser -u ${UID} -S -G ${OPBOT_USER} -s /bin/bash -h ${OPBOT_HOME_DIR} ${OPBOT_USER} && \
    apk update && \
    apk add --no-cache bash coreutils git go libc-dev make wget && \
    mkdir -p "${OPBOT_GIT_ROOT}" && \
    git clone "${GITHUB_URL}" "${OPBOT_GIT_ROOT}" && \
    cd "${OPBOT_GIT_ROOT}" && \
    ls -ltr && \
    cd src && \
    go get -v && \
    cd .. && \
    make && \
    mv "${OPBOT_PROJECT}" /usr/bin && \
    cd /tmp && \
    rm -rf "${GOPATH}" && \
    apk add ca-certificates && \
    apk del coreutils git go libc-dev make wget && \
    rm -rf /var/cache/apk/*

ENTRYPOINT ["/bin/su", "-", "opbot", "-c", "'/usr/bin/op-bot'"]

