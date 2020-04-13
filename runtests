#!/bin/bash
#
# Helper script to run tests:
# - Test results formatted as XML to be compatible with CircleCI.
# - Coverage reports as html for easy overview.
#

set -uxo pipefail
go test -cover -v -race -mod=vendor -coverprofile=cover.out -p 1 ./... | tee tests.out
ret=$?
mkdir -p tests
cat tests.out | go-junit-report > tests/tests.xml
go tool cover -html=cover.out -o artifacts/coverage.html
go tool cover -func=cover.out
exit $ret
