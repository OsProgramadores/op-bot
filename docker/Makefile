PROJECT_NAME=op-bot
DOCKER_COMPOSE=docker-compose --project-name ${PROJECT_NAME}

build:
	${DOCKER_COMPOSE} build

force:
	${DOCKER_COMPOSE} build --no-cache

.PHONY: build force
