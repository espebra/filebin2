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

| Description | Environment Variable | Command Line Argument | Default |
|-------------|---------------------|----------------------|---------|
| The base URL to use, which impacts URLs that are presented to the user for files and bins. It needs to point to the hostname of the filebin instance. | `FILEBIN_BASEURL` | `--baseurl` | `https://filebin.net` |
| The e-mail address to show on the website on the contact page. | `FILEBIN_CONTACT` | `--contact` | (required) |
| Bin expiration time in seconds since the last bin update. Bins will be inaccessible after this time, and files will be removed by the lurker. | `FILEBIN_EXPIRATION` | `--expiration` | `604800` |
| Directory for temporary files for upload and download. | `FILEBIN_TMPDIR` | `--tmpdir` | `/tmp` |
| Workspace capacity threshold multiplier. | `FILEBIN_TMPDIR_CAPACITY_THRESHOLD` | | `4.0` |
| If enabled, the administrator needs to manually approve new bins before files and archives can be downloaded. Bin and file operations except downloading are accepted while a bin is pending approval. This is a mechanism added to limit abuse. The API request used to approve a bin is an authenticated `PUT /admin/approve/{bin}`. | `FILEBIN_MANUAL_APPROVAL` | `--manual-approval` | `false` |
| If enabled, a warning page will be shown before a user can download files. If the user accepts to continue past the warning page, a cookie will be set to identify that the user understands the risk of downloading files. | `FILEBIN_REQUIRE_VERIFICATION_COOKIE` | `--require-verification-cookie` | `false` |
| Number of days before cookie expiration. See `--require-verification-cookie`. | `FILEBIN_VERIFICATION_COOKIE_LIFETIME` | `--verification-cookie-lifetime` | `365` |
| Which cookie value to expect to avoid showing a warning message. See `--require-verification-cookie`. | `FILEBIN_EXPECTED_COOKIE_VALUE` | `--expected-cookie-value` | `2024-05-24` |
| The path to an mmdb formatted geoip database like GeoLite2-City.mmdb. This is optional. | `FILEBIN_MMDB_CITY` | `--mmdb` | (not set) |
| The path to an mmdb formatted geoip database like GeoLite2-ASN.mmdb. This is optional. | `FILEBIN_MMDB_ASN` | | (not set) |
| If enabled, the `X-Robots-Tag` response header will allow search engines to index and show Filebin in search results. Otherwise, robots will be instructed to not show files and bins in search results. | `FILEBIN_ALLOW_ROBOTS` | `--allow-robots` | `false` |

#### Limits

| Description | Environment Variable | Command Line Argument | Default |
|-------------|---------------------|----------------------|---------|
| Limit the number of downloads per file. 0 means no limit. If the value is 100, then each file can be downloaded 100 times before further downloads are rejected. | `FILEBIN_LIMIT_FILE_DOWNLOADS` | `--limit-file-downloads` | `0` |
| Limit the storage capacity that filebin will use. 0 means no limit. If set to "200GB", filebin will allow file uploads until the total storage surpasses 200 GB. New file uploads will be rejected until storage consumption is below 200 GB again. | `FILEBIN_LIMIT_STORAGE` | `--limit-storage` | `0` |
| A whitespace separated list of file extensions that will be rejected. Example: "exe bat dll". | `FILEBIN_REJECT_FILE_EXTENSIONS` | `--reject-file-extensions` | (not set) |

#### HTTP Server

| Description | Environment Variable | Command Line Argument | Default |
|-------------|---------------------|----------------------|---------|
| Which IP address Filebin will bind to. The default value of 127.0.0.1 is a safe default that does not expose Filebin outside of the host. | `FILEBIN_LISTEN_HOST` | `--listen-host` | `127.0.0.1` |
| Which port Filebin will listen to. The default of 8080 does not require privileged access. | `FILEBIN_LISTEN_PORT` | `--listen-port` | `8080` |
| Path to a filename for the access log output. | `FILEBIN_ACCESS_LOG` | `--access-log` | `/var/log/filebin/access.log` |
| If enabled, the client IP will be read from the proxy headers provided in the incoming HTTP requests. This should only be enabled if there is an HTTP proxy running in front of Filebin that uses proxy headers to tell Filebin the original client IP address. | `FILEBIN_PROXY_HEADERS` | `--proxy-headers` | `false` |

#### Timeouts

| Description | Environment Variable | Command Line Argument | Default |
|-------------|---------------------|----------------------|---------|
| Read timeout for the HTTP server. File uploads need to complete within this timeout before they are terminated. | `FILEBIN_READ_TIMEOUT` | `--read-timeout` | `1h` |
| Read header timeout for the HTTP server. | `FILEBIN_READ_HEADER_TIMEOUT` | `--read-header-timeout` | `2s` |
| Write timeout for the HTTP server. | `FILEBIN_WRITE_TIMEOUT` | `--write-timeout` | `1h` |
| Idle timeout for the HTTP server. | `FILEBIN_IDLE_TIMEOUT` | | `30s` |

#### Database

| Description | Environment Variable | Command Line Argument | Default |
|-------------|---------------------|----------------------|---------|
| Which PostgreSQL host to connect to. This can be an IP address or a hostname. | `FILEBIN_DB_HOST` | `--db-host` | (required) |
| The port to use when connecting to the PostgreSQL database. | `FILEBIN_DB_PORT` | `--db-port` | `5432` |
| The name of the PostgreSQL database to use. | `FILEBIN_DB_NAME` | `--db-name` | (required) |
| The username to use when authenticating to the PostgreSQL database. | `FILEBIN_DB_USERNAME` | `--db-username` | (required) |
| The password to use when authenticating to the PostgreSQL database. | `FILEBIN_DB_PASSWORD` | `--db-password` | (required) |
| Maximum number of open database connections. | `FILEBIN_DB_MAX_OPEN_CONNS` | | `25` |
| Maximum number of idle database connections. | `FILEBIN_DB_MAX_IDLE_CONNS` | | `25` |

#### S3 Storage

| Description | Environment Variable | Command Line Argument | Default |
|-------------|---------------------|----------------------|---------|
| The S3 endpoint to connect to. This can be the hostname or IP address. When self hosting S3 on a non-standard port, the port can be specified using `hostname:port`. | `FILEBIN_S3_ENDPOINT` | `--s3-endpoint` | (required) |
| The name of the bucket in S3 where files will be stored. | `FILEBIN_S3_BUCKET` | `--s3-bucket` | (required) |
| The S3 region where the bucket lives. | `FILEBIN_S3_REGION` | `--s3-region` | (required) |
| The access key to use when connecting to the S3 bucket where files will be stored. | `FILEBIN_S3_ACCESS_KEY` | `--s3-access-key` | (required) |
| The secret key to use when connecting to the S3 bucket where files will be stored. | `FILEBIN_S3_SECRET_KEY` | `--s3-secret-key` | (required) |
| Whether or not Filebin will require the connection to S3 to be TLS encrypted using https. If set to false, Filebin will attempt connecting to S3 using plain http. | `FILEBIN_S3_SECURE` | `--s3-secure` | `true` |
| Enable S3 HTTP tracing for debugging. This will provide verbose logging on file uploads. | `FILEBIN_S3_TRACE` | `--s3-trace` | `false` |
| When a Filebin user downloads a file, that is done using a presigned URL that contains a token with limited time to live. The default allows presigned URLs to be used for 1 minute before they expire. The value is specified using the time unit, examples: `30s`, `5m`, `2h`. | `FILEBIN_S3_URL_TTL` | `--s3-url-ttl` | `1m` |
| Timeout for quick S3 operations. | `FILEBIN_S3_TIMEOUT` | | `30s` |
| Timeout for S3 data transfers. | `FILEBIN_S3_TRANSFER_TIMEOUT` | | `10m` |

#### Lurker (Background Jobs)

| Description | Environment Variable | Command Line Argument | Default |
|-------------|---------------------|----------------------|---------|
| The lurker is a batch job that runs automatically and in the background to delete expired bins and remove old log entries from the database. This specifies the time in seconds for the lurker to sleep between each execution. | `FILEBIN_LURKER_INTERVAL` | `--lurker-interval` | `300` |
| Milliseconds to wait between S3 deletions to throttle the deletion rate. | `FILEBIN_LURKER_THROTTLE` | | `250` |
| The number of days to keep transaction log entries in the PostgreSQL database before they are removed by the lurker. | `FILEBIN_LOG_RETENTION` | `--log-retention` | `7` |

#### Admin Authentication

| Description | Environment Variable | Command Line Argument | Default |
|-------------|---------------------|----------------------|---------|
| Username to require for access to the /admin endpoint. If the username is not set, then the admin endpoint will not be available. | `FILEBIN_ADMIN_USERNAME` | `--admin-username` | (not set) |
| Password to require for access to the /admin endpoint. If the password is not set, then the admin endpoint will not be available. Make sure to keep this password a secret. | `FILEBIN_ADMIN_PASSWORD` | `--admin-password` | (not set) |

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
