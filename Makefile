.PHONY: default
default:
	go version

	# Bundle templates and static files into the build
	rm -f templates.rice-box.go
	rm -f static.rice-box.go
	rm -f rice-box.go
	rice embed-go -v -i .

	mkdir -p tests artifacts

	go test -cover -v -mod=vendor -coverprofile=cover.out -p 1 ./... | tee tests.out
	cat tests.out | go-junit-report > tests/tests.xml
	go tool cover -html=cover.out -o artifacts/coverage.html
	go tool cover -func=cover.out

	GOOS=darwin GOARCH=amd64 go build -o artifacts/filebin-darwin-amd64
	GOOS=linux GOARCH=amd64 go build -o artifacts/filebin-linux-amd64

fmt:
	gofmt -w *.go
	gofmt -w ds/*.go
	gofmt -w s3/*.go
	gofmt -w dbl/*.go
