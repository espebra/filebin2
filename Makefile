.PHONY: default
default: test linux

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

prepare:
	go version
	mkdir -p artifacts tests

test: prepare
	bash runtests

linux: prepare
	GOOS=linux GOARCH=amd64 go build -mod=vendor -o artifacts/filebin2-linux-amd64 -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" ./cmd/filebin2

darwin: prepare
	GOOS=darwin GOARCH=amd64 go build -mod=vendor -o artifacts/filebin2-darwin-amd64 -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" ./cmd/filebin2

run: linux
	artifacts/filebin2-linux-amd64 --listen-host 0.0.0.0 --lurker-interval 10 --expiration 3600 --access-log=access.log --s3-secure=false --db-host=db --limit-storage 1G --admin-username admin --admin-password changeme --metrics --metrics-username metrics --metrics-password changeme --mmdb-city mmdb/GeoLite2-City.mmdb --mmdb-asn mmdb/GeoLite2-ASN.mmdb --require-verification-cookie --contact "changeme@filebin.net"

fmt:
	gofmt -w -s cmd/filebin2/*.go
	gofmt -w -s internal/web/*.go
	gofmt -w -s internal/lurker/*.go
	gofmt -w -s internal/ds/*.go
	gofmt -w -s internal/s3/*.go
	gofmt -w -s internal/dbl/*.go
	gofmt -w -s internal/geoip/*.go
	gofmt -w -s internal/workspace/*.go
