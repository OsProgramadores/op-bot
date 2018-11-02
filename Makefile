.PHONY: mod-update clean

BIN := "op-bot"

# Default target
${BIN}: Makefile *.go
	go build

# Cleanup
clean:
	go clean
