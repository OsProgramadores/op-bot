#!/bin/bash
# Test Go formatting compliance with:
# - gofmt
# - golint
# - go vet
#
# Needs:
# golint: go get -u github.com/golang/lint/golint

TMPFILE=$(mktemp)
trap "rm -f $TMPFILE" EXIT

# Run gofmt in all go files. Report diffs.
find . -type f -name '*.go' -exec gofmt -l {} \; >"$TMPFILE"

# Error out if any files need formatting.
if grep -q . "$TMPFILE"; then
  echo "ERROR: The following files need formatting with gofmt:"
  cat $TMPFILE
  exit 1
fi

# From this point on, we exit immediately with any failure.
set -e

echo "Checking source with go-lint."
golint -set_exit_status

echo "Checking source with go-vet."
go vet
