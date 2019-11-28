FROM golang:1.13-alpine3.10 AS builder

MAINTAINER Marco Paganini <paganini@paganini.net> and Sergio Correia <sergio@correia.cc>

ARG UID=501

# Copy the repo contents into /tmp/build (inside the container)
WORKDIR /tmp/build/src/op-bot
COPY . .

# Fully static (as long we we don't need to link to C)
ENV CGO_ENABLED 0

RUN apk add --no-cache ca-certificates git make && \
    export HOME="/tmp" && \
    export GOPATH="/tmp/build" && \
    cd src && \
    go get -u -v "github.com/BurntSushi/toml" && \
    go get -u -v "github.com/nicksnyder/go-i18n/i18n" && \
    go get -u -v "github.com/patrickmn/go-cache" && \
    go get -u -v "github.com/stretchr/testify/mock" && \
    go get -u -v "gopkg.in/telegram-bot-api.v4" && \
    cd .. && \
    make

# Build the second stage (small) image
FROM alpine

# These get exported to the environment at run time.
ENV XDG_CONFIG_HOME "/config"
ENV XDG_DATA_HOME "/data"
ENV TRANSLATIONS_DIR "/app/translations"

WORKDIR /app
COPY --from=builder /tmp/build/src/op-bot/op-bot .
COPY --from=builder /tmp/build/src/op-bot/translations ${TRANSLATIONS_DIR}

# Geo requests API port.
EXPOSE 54321

USER ${UID}
ENTRYPOINT [ "./op-bot" ]

