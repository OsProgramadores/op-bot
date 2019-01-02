#!/bin/bash
# Test Go formatting compliance with:
# - gofmt -s
# - golint
# - go vet
#
# Needs:
# golint: go get -u github.com/golang/lint/golint

TMPFILE=$(mktemp)
trap "rm -f $TMPFILE" EXIT

# Run gofmt in all go files. Report diffs.
echo "Checking source with gofmt -s."
find src -type f -name '*.go' -exec gofmt -s -l {} \; >"$TMPFILE"

# Error out if any files need formatting.
if grep -q . "$TMPFILE"; then
  echo "ERROR: The following files need formatting with gofmt -s:"
  cat $TMPFILE
  exit 1
fi

# From this point on, we exit immediately with any failure.
set -e
cd src

echo "Checking source with go-lint."
golint -set_exit_status

echo "Checking source with go-vet."
go vet

echo "Checking translation IDs."
cd ..
travis-ci/transcheck/transcheck --source-dir "src" --translations-dir "examples/translations"
