FROM alpine:3.17 as builder

MAINTAINER Marco Paganini <paganini@paganini.net> and Sergio Correia <sergio@correia.cc>

ARG UID=501

# Copy the repo contents into /tmp/build (inside the container)
WORKDIR /tmp/build/src/op-bot
COPY . .

# Fully static (as long we we don't need to link to C)
ENV CGO_ENABLED 0

RUN apk add --no-cache ca-certificates git git-crypt go make && \
    export HOME="/tmp" && \
    export GOPATH="/tmp/build" && \
    cd src && \
    go mod download && \
    cd .. && \
    make

# Build the second stage (small) image
FROM alpine

# These get exported to the environment at run time.
ENV XDG_CONFIG_HOME "/config"
ENV XDG_DATA_HOME "/data"
ENV TRANSLATIONS_DIR "/app/translations"
ENV CONFIG_DIR "${XDG_CONFIG_HOME}/op-bot"

WORKDIR /app
COPY --from=builder /tmp/build/src/op-bot/op-bot .
COPY --from=builder /tmp/build/src/op-bot/translations ${TRANSLATIONS_DIR}
COPY --from=builder /tmp/build/src/op-bot/site-configs ${CONFIG_DIR}

# Geo requests API port.
EXPOSE 54321

USER ${UID}
ENTRYPOINT [ "./op-bot" ]
