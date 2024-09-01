.PHONY: default
default: test linux

prepare:
	go version
	mkdir -p artifacts tests

test: prepare
	bash runtests

linux: prepare
	GOOS=linux GOARCH=amd64 go build -mod=vendor -o artifacts/filebin2-linux-amd64 -trimpath -buildvcs=false

darwin: prepare
	GOOS=darwin GOARCH=amd64 go build -mod=vendor -o artifacts/filebin2-darwin-amd64 -trimpath -buildvcs=false

run: linux
	artifacts/filebin2-linux-amd64 --listen-host 0.0.0.0 --lurker-interval 10 --expiration 3600 --access-log=access.log --s3-secure=false --db-host=db --limit-storage 1G --admin-username admin --admin-password changeme --metrics --mmdb-city mmdb/GeoLite2-City.mmdb --mmdb-asn mmdb/GeoLite2-ASN.mmdb --require-verification-cookie --contact "changeme@filebin.net"

fmt:
	gofmt -w -s *.go
	gofmt -w -s ds/*.go
	gofmt -w -s s3/*.go
	gofmt -w -s dbl/*.go
	gofmt -w -s geoip/*.go
