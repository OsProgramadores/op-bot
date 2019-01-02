# Op-bot docker configuration

## Pre-requisites

* [docker](https://www.docker.com/products/docker-engine). If possible, install
  it using your OS native packaging system (under Debian, use `apt-get
  install docker-ce`).

* [docker-compose](https://docs.docker.com/compose/install/).

* GNU Make, which should be installed on most linux distributions by default.

## Building a new image

Create the docker image to run the bot:

```
make
```

This should create a new image with the bot. You should see something like the
following, as the last lines of the process, indicating things went well:

```
Successfully built a06d65011e27
Successfully tagged op-bot_bot:latest
```

Copy the example `../examples/config.toml` file into the `./config` directory.
Edit the file and set your token. There are additional settings, but `token` is
the bare minimum required in order to run the bot.

Copy the translations from the `../examples/translations` directory into the
`./config/translations` directory.

## Running

1. Create the container with:
    ```
    docker-compose -p op-bot down
    docker-compose -p op-bot up
    ```

The `down` operation will clean up any old containers around. The `up` operation
will actually bring the container up.

In case of problems, read the error messages carefully, as they usually explain
what went wrong. Also make sure you did the previous steps correctly.
