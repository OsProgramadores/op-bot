# Running the bot as a docker container

Assuming you already have `docker` (and `docker-compose`) installed and
running, a few other things are needed in order to run the bot as a docker
container:

1. Create the docker image to run the bot:
```
make
```

This should create the image `op-docker/op-bot`. You should see something like
the following, as the last lines of the process, indicating things went well:

```
Successfully built 3848aa863492
Successfully tagged op-docker/op-bot:latest
```

2. Copy the example `../examples/config.toml` file into the `./config`
   directory. Edit the file and set your token. There are additional settings,
   but `token` is the bare minimum required in order to run the bot.

3. Copy the example `../examples/messages.toml` file into the `./config`
   directory.  Edit the mesages, if desired.

4. Create the container with:
```
docker-compose up
```

If things go well, you should see something along these lines:
```
Creating network "docker_default" with the default driver
Creating docker_bot_1 ...
Creating docker_bot_1 ... done
Attaching to docker_bot_1
bot_1  | 2017/07/22 20:18:19 Authorized on account osprogramadores_bot
```

In case of problems, read the error messages carefully, as they usually explain
what went wrong. Also make sure you did the previous steps correctly.

And that's it.
