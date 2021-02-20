[![CircleCI](https://circleci.com/gh/espebra/filebin2/tree/master.svg?style=shield)](https://circleci.com/gh/espebra/filebin2/tree/master) [![Actions](https://github.com/espebra/filebin2/workflows/Actions/badge.svg)](https://github.com/espebra/filebin2/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/espebra/filebin2)](https://goreportcard.com/report/github.com/espebra/filebin2)
[![codecov](https://codecov.io/gh/espebra/filebin2/branch/master/graph/badge.svg)](https://codecov.io/gh/espebra/filebin2)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat)](https://pkg.go.dev/github.com/espebra/filebin2)

# Development environment

The development environment consists of a PostgreSQL database, an MinIO object storage instance and a container running filebin2. The easiest way to set up this environment is to clone this repository and do:

```bash
docker-compose up --build
```

This will make:

* Filebin2 available on [http://localhost:8080/](http://localhost:8080/).
* Filebin2 admin available on [http://admin:changeme@localhost:8080/admin](http://admin:changeme@localhost:8080/).
* MinIO available on [http://localhost:9000/](http://localhost:9000/).
* PostgreSQL available on `localhost:5432`.

