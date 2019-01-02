.PHONY: clean test

bin := op-bot
src := $(wildcard src/*.go)

# Default target
${bin}: Makefile ${src}
	cd src && go build -v -o "../${bin}"

test:
	cd src && go test -v

# Cleanup
clean:
	cd src && go clean
	rm -f "${bin}"
