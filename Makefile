.PHONY: arch clean docker docker-force install test

bin := op-bot
bindir := /usr/local/bin
archdir := arch
src := $(wildcard src/*.go)
git_tag := $(shell git describe --always --tags)

# Default target
${bin}: Makefile ${src}
	cd src && go build -v -ldflags "-X main.BuildVersion=${git_tag}" -o "../${bin}"

install: ${bin}
	install -m 755 "${bin}" "${bindir}"

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
	rm -rf "${archdir}"

# Creates cross-compiled tarred versions (for releases).
arch: Makefile ${src}
	for ga in "linux/amd64" "linux/386" "linux/arm" "linux/arm64" "linux/mips" "linux/mipsle"; do \
	  export GOOS="$${ga%/*}"; \
	  export GOARCH="$${ga#*/}"; \
	  dst="./${archdir}/$${GOOS}-$${GOARCH}"; \
	  mkdir -p "$${dst}"; \
	  go build -v -ldflags "-X main.Build=${git_tag}" -o "$${dst}/${bin}"; \
	  install -m 644 LICENSE README.md "$${dst}"; \
	  tar -C "${archdir}" -zcvf "${archdir}/${bin}-$${GOOS}-$${GOARCH}.tar.gz" "$${dst##*/}"; \
	done
