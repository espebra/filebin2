.PHONY: default
default: test linux

prepare:
	go version
	rm -f templates.rice-box.go static.rice-box.go rice-box.go
	mkdir -p artifacts tests

test: prepare
	bash runtests

linux: prepare
	rice embed-go -v -i .
	GOOS=linux GOARCH=amd64 go build -mod=vendor -o artifacts/filebin2-linux-amd64

darwin: prepare
	rice embed-go -v -i .
	GOOS=darwin GOARCH=amd64 go build -mod=vendor -o artifacts/filebin2-darwin-amd64

run: linux
	artifacts/filebin2-linux-amd64 --listen-host 0.0.0.0 --lurker-interval 10 --expiration 3600 --access-log=foo.log --s3-secure=false --db-host=db

fmt:
	gofmt -w *.go
	gofmt -w ds/*.go
	gofmt -w s3/*.go
	gofmt -w dbl/*.go
