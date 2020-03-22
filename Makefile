.PHONY: default
default:
	go version

	# Bundle templates and static files into the build
	rm -f templates.rice-box.go
	rm -f static.rice-box.go
	rm -f rice-box.go
	rice embed-go -v -i .

	mkdir -p tests artifacts
	go test -cover -v -mod=vendor -coverprofile=cover.out dbl/* | tee dbl.out
	cat dbl.out | go-junit-report > tests/dbl.xml
	go tool cover -html=cover.out -o artifacts/dbl.html
	go tool cover -func=cover.out

	go test -cover -v -mod=vendor -coverprofile=cover.out s3/* | tee s3.out
	cat s3.out | go-junit-report > tests/s3.xml
	go tool cover -html=cover.out -o artifacts/s3.html
	go tool cover -func=cover.out

	GOOS=darwin GOARCH=amd64 go build -o artifacts/filebin-darwin-amd64
	GOOS=linux GOARCH=amd64 go build -o artifacts/filebin-linux-amd64

fmt:
	gofmt -w *.go
	gofmt -w ds/*.go
	gofmt -w s3/*.go
	gofmt -w dbl/*.go
