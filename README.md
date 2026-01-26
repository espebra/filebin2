[![CI](https://github.com/espebra/filebin2/actions/workflows/ci.yaml/badge.svg)](https://github.com/espebra/filebin2/actions/workflows/ci.yaml)
[![Release](https://github.com/espebra/filebin2/actions/workflows/release.yaml/badge.svg)](https://github.com/espebra/filebin2/actions/workflows/release.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/espebra/filebin2)](https://goreportcard.com/report/github.com/espebra/filebin2)
[![codecov](https://codecov.io/gh/espebra/filebin2/branch/master/graph/badge.svg)](https://codecov.io/gh/espebra/filebin2)
[![CodeQL](https://github.com/espebra/filebin2/actions/workflows/github-code-scanning/codeql/badge.svg?branch=master)](https://github.com/espebra/filebin2/actions/workflows/github-code-scanning/codeql)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat)](https://pkg.go.dev/github.com/espebra/filebin2)

Filebin2 is a web application that facilitates convenient file sharing over the web. It is the software that powers https://filebin.net. It is still in development status and will see breaking changes.

## Table of contents

* [Why filebin2?](#why-filebin2)
* [Development environment](#development-environment)
* [Usage](#usage)
  * [Configuration](#configuration)
* [Integrations](#integrations)

## Why filebin2?

A couple of (in hindsight) bad architectural decisions in the [previous version of filebin](https://github.com/espebra/filebin) paved the road for filebin2. Filebin2 is using a PostgreSQL database to handle meta data and S3 to store files. I decided to move to a new repository because of breaking changes from the previous verson of filebin.

## Development environment

The development environment consists of one PostgreSQL instance, one [Stupid Simple S3](https://github.com/espebra/stupid-simple-s3) object storage instance and an instance of filebin2. The easiest way to set up this environment from source code is to clone this repository and do:

```bash
# With Docker
docker compose up --build

# With Podman
podman compose up --build
```

This will make:

* Filebin2 available on [http://localhost:8080/](http://localhost:8080/).
* Filebin2 admin available on [http://admin:changeme@localhost:8080/admin](http://admin:changeme@localhost:8080/admin).
* Stupid Simple S3 available on [http://localhost:5553/](http://localhost:5553/).
* PostgreSQL available on `localhost:5432`.

## Usage

Filebin can run in most Linux distributions, and most likely other operating systems like MacOS. It runs fine containerized.

Filebin requires read write access to an S3 bucket for file storage and a PostgreSQL database that it will use for meta data.

The Filebin program itself is written in Go and builds to a single binary that is configured using command line arguments.

### Testing and building

The easiest way to run the test suite is to run it in docker compose. Docker will exit successfully (return code 0) if the tests succeed, and exit with an error code other than 0 if the tests fail.

```bash
# Docker
docker compose -f integration-tests.yml up --abort-on-container-exit

# Podman
podman compose -f integration-tests.yml up --abort-on-container-exit
```

The program can be built using:

```bash
# Build for Linux amd64 only
make linux

# Build for all platforms:
make build-all
```

The output will be the Filebin program as binaries in the `artifacts/` folder called `filebin2-linux-amd64` (depending on the platform). The filebin binary take the following environment variables and command line parameters.

### Configuration

Filebin can be configured using command line arguments or environment variables. Environment variables use the `FILEBIN_` prefix with uppercase letters and underscores instead of hyphens. Command line flags take precedence over environment variables.

#### General

| Environment Variable | Command Line Argument | Description | Default |
|---------------------|----------------------|-------------|---------|
| `FILEBIN_BASEURL` | `--baseurl` | The base URL to use, which impacts URLs that are presented to the user for files and bins. It needs to point to the hostname of the filebin instance. | `https://filebin.net` |
| `FILEBIN_CONTACT` | `--contact` | The e-mail address to show on the website on the contact page. | (required) |
| `FILEBIN_EXPIRATION` | `--expiration` | Bin expiration time in seconds since the last bin update. Bins will be inaccessible after this time, and files will be removed by the lurker. | `604800` |
| `FILEBIN_TMPDIR` | `--tmpdir` | Directory for temporary files for upload and download. | `/tmp` |
| `FILEBIN_TMPDIR_CAPACITY_THRESHOLD` | | Workspace capacity threshold multiplier. | `4.0` |
| `FILEBIN_MANUAL_APPROVAL` | `--manual-approval` | If enabled, the administrator needs to manually approve new bins before files and archives can be downloaded. Bin and file operations except downloading are accepted while a bin is pending approval. This is a mechanism added to limit abuse. The API request used to approve a bin is an authenticated `PUT /admin/approve/{bin}`. | `false` |
| `FILEBIN_REQUIRE_VERIFICATION_COOKIE` | `--require-verification-cookie` | If enabled, a warning page will be shown before a user can download files. If the user accepts to continue past the warning page, a cookie will be set to identify that the user understands the risk of downloading files. | `false` |
| `FILEBIN_VERIFICATION_COOKIE_LIFETIME` | `--verification-cookie-lifetime` | Number of days before cookie expiration. See `--require-verification-cookie`. | `365` |
| `FILEBIN_EXPECTED_COOKIE_VALUE` | `--expected-cookie-value` | Which cookie value to expect to avoid showing a warning message. See `--require-verification-cookie`. | `2024-05-24` |
| `FILEBIN_MMDB_CITY` | `--mmdb` | The path to an mmdb formatted geoip database like GeoLite2-City.mmdb. This is optional. | (not set) |
| `FILEBIN_MMDB_ASN` | | The path to an mmdb formatted geoip database like GeoLite2-ASN.mmdb. This is optional. | (not set) |
| `FILEBIN_ALLOW_ROBOTS` | `--allow-robots` | If enabled, the `X-Robots-Tag` response header will allow search engines to index and show Filebin in search results. Otherwise, robots will be instructed to not show files and bins in search results. | `false` |

#### Limits

| Environment Variable | Command Line Argument | Description | Default |
|---------------------|----------------------|-------------|---------|
| `FILEBIN_LIMIT_FILE_DOWNLOADS` | `--limit-file-downloads` | Max downloads per file (0 = unlimited) | `0` |
| `FILEBIN_LIMIT_STORAGE` | `--limit-storage` | Storage capacity limit (e.g., `100GB`, 0 = unlimited) | `0` |
| `FILEBIN_REJECT_FILE_EXTENSIONS` | `--reject-file-extensions` | Space-separated list of rejected extensions | (not set) |

#### HTTP Server

| Environment Variable | Command Line Argument | Description | Default |
|---------------------|----------------------|-------------|---------|
| `FILEBIN_LISTEN_HOST` | `--listen-host` | IP address to bind to | `127.0.0.1` |
| `FILEBIN_LISTEN_PORT` | `--listen-port` | Port to listen on | `8080` |
| `FILEBIN_ACCESS_LOG` | `--access-log` | Path to access log file | `/var/log/filebin/access.log` |
| `FILEBIN_PROXY_HEADERS` | `--proxy-headers` | Read client IP from proxy headers | `false` |

#### Timeouts

| Environment Variable | Command Line Argument | Description | Default |
|---------------------|----------------------|-------------|---------|
| `FILEBIN_READ_TIMEOUT` | `--read-timeout` | HTTP read timeout (uploads must complete within) | `1h` |
| `FILEBIN_READ_HEADER_TIMEOUT` | `--read-header-timeout` | HTTP read header timeout | `2s` |
| `FILEBIN_WRITE_TIMEOUT` | `--write-timeout` | HTTP write timeout | `1h` |
| `FILEBIN_IDLE_TIMEOUT` | | HTTP idle timeout | `30s` |

#### Database

| Environment Variable | Command Line Argument | Description | Default |
|---------------------|----------------------|-------------|---------|
| `FILEBIN_DB_HOST` | `--db-host` | PostgreSQL host (IP or hostname) | (required) |
| `FILEBIN_DB_PORT` | `--db-port` | PostgreSQL port | `5432` |
| `FILEBIN_DB_NAME` | `--db-name` | Database name | (required) |
| `FILEBIN_DB_USERNAME` | `--db-username` | Database username | (required) |
| `FILEBIN_DB_PASSWORD` | `--db-password` | Database password | (required) |
| `FILEBIN_DB_MAX_OPEN_CONNS` | | Max open database connections | `25` |
| `FILEBIN_DB_MAX_IDLE_CONNS` | | Max idle database connections | `25` |

#### S3 Storage

| Environment Variable | Command Line Argument | Description | Default |
|---------------------|----------------------|-------------|---------|
| `FILEBIN_S3_ENDPOINT` | `--s3-endpoint` | S3 endpoint (host or host:port) | (required) |
| `FILEBIN_S3_BUCKET` | `--s3-bucket` | S3 bucket name | (required) |
| `FILEBIN_S3_REGION` | `--s3-region` | S3 region | (required) |
| `FILEBIN_S3_ACCESS_KEY` | `--s3-access-key` | S3 access key | (required) |
| `FILEBIN_S3_SECRET_KEY` | `--s3-secret-key` | S3 secret key | (required) |
| `FILEBIN_S3_SECURE` | `--s3-secure` | Use TLS for S3 connection | `true` |
| `FILEBIN_S3_TRACE` | `--s3-trace` | Enable S3 debug tracing | `false` |
| `FILEBIN_S3_URL_TTL` | `--s3-url-ttl` | Presigned URL time-to-live (e.g., `30s`, `5m`) | `1m` |
| `FILEBIN_S3_TIMEOUT` | | Timeout for quick S3 operations | `30s` |
| `FILEBIN_S3_TRANSFER_TIMEOUT` | | Timeout for S3 data transfers | `10m` |

#### Lurker (Background Jobs)

| Environment Variable | Command Line Argument | Description | Default |
|---------------------|----------------------|-------------|---------|
| `FILEBIN_LURKER_INTERVAL` | `--lurker-interval` | Seconds between cleanup runs | `300` |
| `FILEBIN_LURKER_THROTTLE` | | Milliseconds between S3 deletions | `250` |
| `FILEBIN_LOG_RETENTION` | `--log-retention` | Days to keep transaction log entries | `7` |

#### Admin Authentication

| Environment Variable | Command Line Argument | Description | Default |
|---------------------|----------------------|-------------|---------|
| `FILEBIN_ADMIN_USERNAME` | `--admin-username` | Admin username for `/admin` endpoint | (not set) |
| `FILEBIN_ADMIN_PASSWORD` | `--admin-password` | Admin password for `/admin` endpoint | (not set) |

#### Metrics

| Environment Variable | Command Line Argument | Description | Default |
|---------------------|----------------------|-------------|---------|
| `FILEBIN_METRICS` | `--metrics` | Enable the `/metrics` endpoint | `false` |
| `FILEBIN_METRICS_USERNAME` | `--metrics-username` | Metrics endpoint username | (not set) |
| `FILEBIN_METRICS_PASSWORD` | `--metrics-password` | Metrics endpoint password | (not set) |
| `FILEBIN_METRICS_AUTH` | `--metrics-auth` | Metrics auth type (e.g., `basic`) | (not set) |
| `FILEBIN_METRICS_ID` | `--metrics-id` | Metrics instance identifier | `$HOSTNAME` |
| `FILEBIN_METRICS_PROXY_URL` | `--metrics-proxy-url` | URL to proxy metrics from | (not set) |

#### Slack Integration

| Environment Variable | Command Line Argument | Description | Default |
|---------------------|----------------------|-------------|---------|
| `FILEBIN_SLACK_SECRET` | `--slack-secret` | Slack webhook secret (required for integration) | (not set) |
| `FILEBIN_SLACK_DOMAIN` | `--slack-domain` | Allowed Slack domain | (not set) |
| `FILEBIN_SLACK_CHANNEL` | `--slack-channel` | Allowed Slack channel | (not set) |

## Integrations

### Grafana and Prometheus

Filebin2 comes with a `/metrics` endpoint that is compatible with Prometheus. There is an [example dashboard](integrations/grafana/filebin.json) that visualizes this data.

![Dashboard](integrations/grafana/screenshot.png)

### Slack

This integration may be useful if manual approval is required (see `--manual-approval`). The integration allows members of a Slack channel to list the recently updated bins and approve specific bins directly in the Slack channel using slash commands. The slash commands available are:

| Slash command | Description |
| ------------- | ----------- |
| `/filebin approve bin_id` | Approve the bin bin_id |
| `/filebin lastupdated` | List the 10 last updated bins |
| `/filebin lastupdated n` | List the n last updated bins |

The documention on how to configure Slack to work with this integration does not exist currently.
