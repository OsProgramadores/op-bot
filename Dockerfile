# Defaults
ARG project_name="op-bot"
ARG project_title="OsProgramadores Telegram bot (op-bot)"
ARG project_author="paganini@paganini.net"
ARG project_source="https://github.com/osprogramadores/op-bot"
ARG project_user="op"
ARG project_uid=501

ARG home="/home/${project_user}"
ARG gopath="${home}/go"
ARG src_dir="${home}/${project_name}"

# Primary build stage.
FROM alpine:3.20 as builder

# Pull from defaults.
ARG project_name
ARG project_user
ARG project_uid
ARG home
ARG gopath
ARG src_dir

# Fully static (as long we we don't need to link to C)
ENV CGO_ENABLED 0

# Directories & PATH
ENV HOME="${home}"
ENV GOPATH="${gopath}"

workdir ${src_dir}
COPY . .

RUN apk add --no-cache ca-certificates git git-crypt go make && \
    mkdir -p "${gopath}" && \
    mkdir -p "${src_dir}" && \
    cd "${src_dir}" && \
    go mod download && \
    make

# Build the second stage (small) image
FROM alpine:3.20
LABEL org.opencontainers.image.title="${project_title}"
LABEL org.opencontainers.image.authors="${project_author}"
LABEL org.opencontainers.image.source="${project_source}"

# Pull ARGs from previous stage.
ARG project_uid
ARG src_dir

# These variables are directly used by the bot.
ENV XDG_CONFIG_HOME "/config"
ENV XDG_DATA_HOME "/data"
ENV TRANSLATIONS_DIR "/app/translations"
ENV CONFIG_DIR "${XDG_CONFIG_HOME}/op-bot"

RUN adduser --uid "${project_uid}" --home /tmp --no-create-home --disabled-password op

WORKDIR /app
COPY --from=builder ${src_dir}/op-bot .
COPY --from=builder ${src_dir}/translations ${TRANSLATIONS_DIR}
COPY --from=builder ${src_dir}/site-configs ${CONFIG_DIR}

# Geo requests API port.
EXPOSE 54321

USER ${project_uid}
ENTRYPOINT [ "./op-bot" ]
