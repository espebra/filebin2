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
  * [Command line arguments](#command-line-arguments)
  * [Environment variables](#environment-variables)
* [Integrations](#integrations)

## Why filebin2?

A couple of (in hindsight) bad architectural decisions in the [previous version of filebin](https://github.com/espebra/filebin) paved the road for filebin2. Filebin2 is using a PostgreSQL database to handle meta data and S3 to store files. I decided to move to a new repository because of breaking changes from the previous verson of filebin.

## Development environment

The development environment consists of one PostgreSQL instance, one MinIO object storage instance and an instance of filebin2. The easiest way to set up this environment is to clone this repository and do:

```bash
docker-compose up --build
```

This will make:

* Filebin2 available on [http://localhost:8080/](http://localhost:8080/).
* Filebin2 admin available on [http://admin:changeme@localhost:8080/admin](http://admin:changeme@localhost:8080/admin).
* MinIO available on [http://localhost:9000/](http://localhost:9000/).
* PostgreSQL available on `localhost:5432`.

## Usage

Filebin can run in most Linux distributions, and most likely other operating systems like MacOS. It runs fine in Docker, but doesn't need to run in Docker.

Filebin requires read write access to an S3 bucket for file storage and a PostgreSQL database that it will use for meta data.

The Filebin program itself is written in Go and builds to a single binary that is configured using command line arguments.

### Testing and building

The easiest way to run the test suite is to run it in docker compose. Docker will exit successfully (return code 0) if the tests succeed, and exit with an error code other than 0 if the tests fail.

```bash
docker-compose -f ci.yml up --abort-on-container-exit
```

The program can be built using:

```bash
make linux
```

The output will be the Filebin program as a single binary in the `artifacts/` folder called `filebin2-linux-amd64`. This binary takes the command line arguments listed below.

### Command line arguments

#### `--access-log string` (default: "/var/log/filebin/access.log")

Path to a filename for the access log output.

#### `--admin-password string` (default: not set)

Password to require for access to the /admin endpoint. If the password is not set, then the admin endpoint will not be available. Make sure to keep this password a secret. Can also be set using the environment variable `FILEBIN_ADMIN_PASSWORD`.

#### `--admin-username string` (default: not set)

Username to require for access to the /admin endpoint. If the password is not set, then the admin endpoint will not be available. Can also be set using the environment variable `FILEBIN_ADMIN_USERNAME`.

#### `--allow-robots` (default: not set)

If this argument is set, then the `X-Robots-Tag` response header will allow search engines to index and show Filebin in search results. Otherwise, robots will be instructed to not show files and bins in search results.

#### `--baseurl string` (default: "https://filebin.net")

The base URL to use, which impacts URLs that are presented to the user for files and bins, and it needs to point to the hostname of the filebin instance.

#### `--contact string` (default: none)

The e-mail address to show on the website on the contact page.

#### `--db-host string` (default: none)

Which PostgreSQL host to connect to. This can be an IP address or a hostname.

#### `--db-name string` (default: none)

The name of the PostgreSQL database to use.

#### `--db-password string` (default: none)

The password to use when authenticating to the PostgreSQL database. Can also be set using the environment variable `FILEBIN_DB_PASSWORD`.

#### `--db-port string` (default: none)

The port to use when connecting to the PostgreSQL database.

#### `--db-username string` (default: none)

The username to use when authenticating to the PostgreSQL database. Can also be set using the environment variable `FILEBIN_DB_USERNAME`.

#### `--expected-cookie-value string` (default: "2024-05-24")

Which cookie value to expect to avoid showing a warning message. See --require-verification-cookie.

#### `--expiration int` (default: 604800)

Bin expiration time in seconds since the last bin update. Bins will be inaccessible after this time, and files will be removed by the lurker (see `--lurker-interval`).

#### `--limit-file-downloads uint` (default: disabled)

This argument can be used to limit the number of downloads per file. 0, which is default, means no limit. If the value is 100, then each file can be downloaded 100 times before further downloads are rejected.

#### `--limit-storage string` (default: disabled)

This argument can be used to limit the storage capacity that filebin will use. 0, which is default, means no limit. If the value is set to "200GB", then filebin will allow file uploads until the total amount of storage capacity used by files uploaded to filebin surpass 200 GB. New file uploads will be rejected until storage consumption is below 200 GB again (see `--lurker-interval`).

#### `--listen-host string` (default: "127.0.0.1")

Which IP address Filebin will bind to. The default value of 127.0.0.1 is a safe default that does not expose Filebin outside of the host.

#### `--listen-port int` (default: 8080)

Which port Filebin will listen to. The default of 8080 does not require privileged access.

#### `--log-retention uint` (default: 7)

The number of days to keep transaction log entries in the PostgreSQL database before they are removed by the lurker (see `--lurker-interval`).

#### `--lurker-interval int` (default: 300)

The lurker is a batch job that runs automatically and in the background to delete expired bins and remove old log entries from the database. This argument is used to specify the time to for the lurker to sleep between in between each execution. The value is specified in seconds.

#### `--manual-approval` (default: not set)

If this argument is set, then the administrator needs to manually approve new bins before files and archives can be downloaded. Bin and file operations except downloading are accepted while a bin is pending approval. This is a mechanism added to limit abuse.

The API request used to approve a bin is an authenticated `PUT /admin/approve/{bin}`

#### `--metrics-username` (default: not set)

The username used for authentication to the `/metrics` endpoint for Prometheus metrics. If the username is not set, this endpoint is disabled.

#### `--metrics-password` (default: not set)

The password used for authentication to the `/metrics` endpoint for Prometheus metrics. If the username is not set, this endpoint is disabled.

#### `--metrics` (default: false)

Enables the `/metrics` endpoint. If this is not set, the endpoint will not return any metrics.

#### `--metrics-auth` (default: not set)

Enables authentication. Currently only basic auth is supported. If `--metrics-auth` or `FILEBIN_METRICS_AUTH` is set to `basic` basic auth will be in play. If not, the endpoint is open to the world.

#### `--metrics-id` (default: hostname)

The string used as the identification of the filebin instance in the Prometheus metrics. By default, this string is the `$HOSTNAME` environment variable.

#### `--metrics-proxy-url` (default: not set)

When this argument is set, Filebin will fetch the content from the URL specified and merge it with its own output on the `/metrics` endpoint. This can be useful when running another Prometheus exporter in the same operating system instance, for example to capture system metrics.

#### `--mmdb string` (default: not set)

The path to an mmdb formatted geoip database like GeoLite2-City.mmdb. This is optional.

#### `--proxy-headers` (default: not set)

If this argument is set, then the client IP will be read from the proxy headers provided in the incoming HTTP requests. This argument should only be set if there is an HTTP proxy running in front of Filebin, that is using the proxy headers to tell Filebin the original client IP address.

#### `--read-header-timeout duration` (default: 2s)

Read header timeout for the HTTP server.

#### `--read-timeout duration` (default: 1h)

Read timeout for the HTTP server. File uploads need to complete within this timeout before they are terminated.

#### `--reject-file-extensions string` (default: not set)

A whitespace separated list of file extensions that will be rejected. Example: "exe bat dll".

#### `--require-verification-cookie` (default not set)

If enabled, a warning page will be shown before a user can download files. If the user accepts to continue past the warning page, a cookie will be set to identify that the user understands the risk of downloading files.

#### `--s3-access-key string` (default: not set)

The access key to use when connecting to the S3 bucket where files will be stored.

#### `--s3-bucket string` (default: not set)

The name of the bucket in S3 where files will be stored.

#### `--s3-endpoint string` (default: not set)

The S3 endpoint to connect to. This can be the hostname or IP address. When self hosting S3 on a non-standard port, the port can be specified using `hostname:port`.

#### `--s3-region string` (default: not set)

The S3 region where the bucket lives.

#### `-s3-secret-key string` (default: not set)

The secret key to use when connecting to the S3 bucket where files will be stored.

#### `--s3-secure` (default: true)

Whether or not Filebin will require the connection to S3 to be TLS encrypted using https. If this parameter is set to false, then Filebin will attempt connecting to S3 using plain http.

#### `--s3-trace` (default: not set)

Enable S3 HTTP tracing for debugging. This will provide verbose logging on file uploads.

#### `--s3-url-ttl string` (default: "1m")

When a Filebin user downloads a file, that is done using a presigned URL that contains a token with limited time to live. The default allows presigned URLs to be used for 1 minute before they expire. The value is specified using the time unit, and some examples are `30s`, `5m` and `2h`.

#### `--slack-channel string` (default: none)

If Filebin is set to require manual approval of new bins (see `--manual-approval`), then this approval can be given using the user interface, the http api directly or via Slack (using the http api).

The http endpoint `/integration/slack` can be accessed using a webhook from Slack.

This parameter limits which Slack channel that is allowed to access the http api. Requests from other channels will be rejected.

#### `--slack-domain string` (default: not set)

This argument limits which Slack domain that is allowed to access the http api. Other domains will be rejected.

#### `--slack-secret string` (default: not set)

This argument specifies the secret that Slack will need to use when connecting to the http api. If this secret is not set, the Slack integration will be disabled.

#### `--tmpdir string`

Directory for temporary files for upload and download (default "`/tmp`").

#### `--verification-cookie-lifetime int` (default: 365)

Number of days before cookie expiration. See --require-verification-cookie.

#### `--write-timeout duration` (default: 1h)

Write timeout for the HTTP server.

### Environment variables

All configuration options can be set via environment variables with the `FILEBIN_` prefix. Environment variables use uppercase letters with underscores instead of hyphens. Command line flags take precedence over environment variables.

#### General

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `FILEBIN_CONTACT` | Contact information (e.g., email address) | (required) |
| `FILEBIN_EXPIRATION` | Bin expiration time in seconds | `604800` |
| `FILEBIN_TMPDIR` | Directory for temporary files | System temp dir |
| `FILEBIN_TMPDIR_CAPACITY_THRESHOLD` | Workspace capacity threshold multiplier | `4.0` |
| `FILEBIN_BASEURL` | Base URL for the instance | `https://filebin.net` |
| `FILEBIN_MANUAL_APPROVAL` | Require manual approval of bins (`true`/`false`) | `false` |
| `FILEBIN_REQUIRE_VERIFICATION_COOKIE` | Require verification cookie (`true`/`false`) | `false` |
| `FILEBIN_VERIFICATION_COOKIE_LIFETIME` | Cookie lifetime in days | `365` |
| `FILEBIN_EXPECTED_COOKIE_VALUE` | Expected cookie value | `2024-05-24` |
| `FILEBIN_MMDB_CITY` | Path to GeoLite2-City.mmdb | (not set) |
| `FILEBIN_MMDB_ASN` | Path to GeoLite2-ASN.mmdb | (not set) |
| `FILEBIN_ALLOW_ROBOTS` | Allow search engine indexing (`true`/`false`) | `false` |

#### Limits

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `FILEBIN_LIMIT_FILE_DOWNLOADS` | Max downloads per file (0 = unlimited) | `0` |
| `FILEBIN_LIMIT_STORAGE` | Storage capacity limit (e.g., `100GB`) | `0` |
| `FILEBIN_REJECT_FILE_EXTENSIONS` | Space-separated list of rejected extensions | (not set) |

#### HTTP Server

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `FILEBIN_LISTEN_HOST` | IP address to bind to | `127.0.0.1` |
| `FILEBIN_LISTEN_PORT` | Port to listen on | `8080` |
| `FILEBIN_ACCESS_LOG` | Path to access log file | `/var/log/filebin/access.log` |
| `FILEBIN_PROXY_HEADERS` | Read client IP from proxy headers (`true`/`false`) | `false` |

#### Timeouts

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `FILEBIN_READ_TIMEOUT` | HTTP read timeout | `1h` |
| `FILEBIN_READ_HEADER_TIMEOUT` | HTTP read header timeout | `2s` |
| `FILEBIN_WRITE_TIMEOUT` | HTTP write timeout | `1h` |
| `FILEBIN_IDLE_TIMEOUT` | HTTP idle timeout | `30s` |

#### Database

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `FILEBIN_DB_HOST` | PostgreSQL host | (required) |
| `FILEBIN_DB_PORT` | PostgreSQL port | `5432` |
| `FILEBIN_DB_NAME` | Database name | (required) |
| `FILEBIN_DB_USERNAME` | Database username | (required) |
| `FILEBIN_DB_PASSWORD` | Database password | (required) |
| `FILEBIN_DB_MAX_OPEN_CONNS` | Max open database connections | `25` |
| `FILEBIN_DB_MAX_IDLE_CONNS` | Max idle database connections | `25` |

#### S3 Storage

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `FILEBIN_S3_ENDPOINT` | S3 endpoint (host or host:port) | (required) |
| `FILEBIN_S3_BUCKET` | S3 bucket name | (required) |
| `FILEBIN_S3_REGION` | S3 region | (required) |
| `FILEBIN_S3_ACCESS_KEY` | S3 access key | (required) |
| `FILEBIN_S3_SECRET_KEY` | S3 secret key | (required) |
| `FILEBIN_S3_SECURE` | Use TLS for S3 (`true`/`false`) | `true` |
| `FILEBIN_S3_TRACE` | Enable S3 debug tracing (`true`/`false`) | `false` |
| `FILEBIN_S3_URL_TTL` | Presigned URL time-to-live | `1m` |
| `FILEBIN_S3_TIMEOUT` | Timeout for quick S3 operations | `30s` |
| `FILEBIN_S3_TRANSFER_TIMEOUT` | Timeout for S3 data transfers | `10m` |

#### Lurker (Background Jobs)

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `FILEBIN_LURKER_INTERVAL` | Seconds between cleanup runs | `300` |
| `FILEBIN_LURKER_THROTTLE` | Milliseconds between S3 deletions | `250` |
| `FILEBIN_LOG_RETENTION` | Days to keep log entries | `7` |

#### Authentication

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `FILEBIN_ADMIN_USERNAME` | Admin username | (not set) |
| `FILEBIN_ADMIN_PASSWORD` | Admin password | (not set) |
| `FILEBIN_METRICS_USERNAME` | Metrics endpoint username | (not set) |
| `FILEBIN_METRICS_PASSWORD` | Metrics endpoint password | (not set) |
| `FILEBIN_METRICS` | Enable metrics endpoint (`true`/`false`) | `false` |
| `FILEBIN_METRICS_AUTH` | Metrics auth type (e.g., `basic`) | (not set) |
| `FILEBIN_METRICS_ID` | Metrics instance identifier | `$HOSTNAME` |
| `FILEBIN_METRICS_PROXY_URL` | URL to proxy metrics from | (not set) |

#### Slack Integration

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `FILEBIN_SLACK_SECRET` | Slack webhook secret | (not set) |
| `FILEBIN_SLACK_DOMAIN` | Allowed Slack domain | (not set) |
| `FILEBIN_SLACK_CHANNEL` | Allowed Slack channel | (not set) |

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
