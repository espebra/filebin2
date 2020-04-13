.PHONY: default
default: test build

prepare:
	go version
	rm -f templates.rice-box.go static.rice-box.go rice-box.go
	mkdir -p artifacts tests

test: prepare
	bash runtests

build: prepare
	rice embed-go -v -i .
	GOOS=darwin GOARCH=amd64 go build -o artifacts/filebin-darwin-amd64
	GOOS=linux GOARCH=amd64 go build -o artifacts/filebin-linux-amd64

fmt:
	gofmt -w *.go
	gofmt -w ds/*.go
	gofmt -w s3/*.go
	gofmt -w dbl/*.go
