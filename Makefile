default:
	go version
	mkdir tests artifacts
	go test -cover -v -mod=vendor -coverprofile=cover.out dbl/* 2>&1 | go-junit-report > tests/go.xml
	go tool cover -func=cover.out
	go tool cover -html=cover.out -o artifacts/coverage.html
	GOOS=darwin GOARCH=amd64 go build -o artifacts/filebin-darwin-amd64
	GOOS=linux GOARCH=amd64 go build -o artifacts/filebin-linux-amd64

