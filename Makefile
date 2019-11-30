.PHONY: clean test docker

bin := op-bot
src := $(wildcard src/*.go)

# Default target
${bin}: Makefile ${src}
	cd src && go build -v -o "../${bin}"

docker:
	docker build -t ${USER}/op-bot .

docker-force:
	docker build --no-cache -t ${USER}/op-bot .

test:
	cd src && go test -v

# Cleanup
clean:
	cd src && go clean
	rm -f "${bin}"
