default:
	go version
	go test -cover -v -mod=vendor -coverprofile=cover.out dbl/* 2>&1 | go-junit-report > out/tests.xml
	go tool cover -func=cover.out
	go tool cover -html=cover.out -o out/coverage.html
	GOOS=darwin GOARCH=amd64 go build -o out/filebin-darwin-amd64
	GOOS=linux GOARCH=amd64 go build -o out/filebin-linux-amd64

