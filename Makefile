.PHONY: default
default: test build

clean:
	rm -f templates.rice-box.go static.rice-box.go rice-box.go

test: clean
	go version
	mkdir -p tests
	go test -cover -v -race -mod=vendor -coverprofile=cover.out -p 1 ./...
	go tool cover -html=cover.out -o artifacts/coverage.html
	go tool cover -func=cover.out

build: clean
	go version
	mkdir -p artifacts
	rm -f templates.rice-box.go static.rice-box.go rice-box.go
	rice embed-go -v -i .
	GOOS=darwin GOARCH=amd64 go build -o artifacts/filebin-darwin-amd64
	GOOS=linux GOARCH=amd64 go build -o artifacts/filebin-linux-amd64

fmt:
	gofmt -w *.go
	gofmt -w ds/*.go
	gofmt -w s3/*.go
	gofmt -w dbl/*.go
