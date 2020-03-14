default:
	go version

	# Bundle templates and static files into the build
	rm -f templates.rice-box.go
	rm -f static.rice-box.go
	rice embed-go -v -i .

	mkdir -p tests artifacts
	go test -cover -v -mod=vendor -coverprofile=cover.out dbl/* 2>&1 > tests/go.out
	cat tests/go.out | go-junit-report > tests/go.xml
	go tool cover -html=cover.out -o artifacts/coverage.html
	go tool cover -func=cover.out
	cat tests/go.out

	GOOS=darwin GOARCH=amd64 go build -o artifacts/filebin-darwin-amd64
	GOOS=linux GOARCH=amd64 go build -o artifacts/filebin-linux-amd64
