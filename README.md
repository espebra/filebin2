[![CircleCI](https://circleci.com/gh/espebra/filebin2/tree/master.svg?style=shield)](https://circleci.com/gh/espebra/filebin2/tree/master) [![Actions](https://github.com/espebra/filebin2/workflows/Actions/badge.svg)](https://github.com/espebra/filebin2/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/espebra/filebin2)](https://goreportcard.com/report/github.com/espebra/filebin2)
[![codecov](https://codecov.io/gh/espebra/filebin2/branch/master/graph/badge.svg)](https://codecov.io/gh/espebra/filebin2)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat)](https://pkg.go.dev/github.com/espebra/filebin2)

# Development environment

```bash
# Install dependency
go get github.com/GeertJohan/go.rice/rice

# Start the database and run the test suite
docker-compose up --force-recreate

# Bundle the templates and static files
rice embed-go -v -i .

# Build the app locally
go build -mod=vendor

# Load the database settings
source env.sh

# Start the app, which will connect to the database in docker
./filebin2
```

