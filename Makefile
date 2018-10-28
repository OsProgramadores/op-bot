.PHONY: mod-update clean

BIN := "op-bot"

# Default target
${BIN}: Makefile go.mod *.go
	GO111MODULE=on go build

# Cleanup
clean:
	rm ${BIN}

# Update modules. Warning: This will reset all module
# versions in go.mod to the latest versions.
mod-update:
	GO111MODULE=on go get -u ./...
	GO111MODULE=on go mod tidy
