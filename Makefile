.PHONY: arch clean docker docker-force install test

BIN := op-bot
BINDIR := /usr/local/bin
ARCHDIR := arch
SRC := $(wildcard src/*.go)
GIT_TAG := $(shell git describe --always --tags)

# Default target
${BIN}: Makefile ${SRC}
	cd src && go build -v -ldflags "-X main.BuildVersion=${GIT_TAG}" -o "../${BIN}"

install: ${BIN}
	install -m 755 "${BIN}" "${BINDIR}"

docker:
	docker build -t ${BIN}:latest .

test:
	cd src && go test -v

# Cleanup
clean:
	cd src && go clean || true
	rm -f "${BIN}"
	rm -rf "${ARCHDIR}"
	docker builder prune -f

# Creates cross-compiled tarred versions (for releases).
arch: Makefile ${SRC}
	for ga in "linux/amd64" "linux/386" "linux/arm" "linux/arm64" "linux/mips" "linux/mipsle"; do \
	  export goos="$${ga%/*}"; \
	  export goarch="$${ga#*/}"; \
	  dst="./${ARCHDIR}/$${goos}-$${goarch}"; \
	  mkdir -p "$${dst}"; \
	  go build -v -ldflags "-X main.Build=${GIT_TAG}" -o "$${dst}/${BIN}"; \
	  install -m 644 LICENSE README.md "$${dst}"; \
	  tar -C "${ARCHDIR}" -zcvf "${ARCHDIR}/${BIN}-$${goos}-$${goarch}.tar.gz" "$${dst##*/}"; \
	done
