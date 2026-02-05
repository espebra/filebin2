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

**Base URL**
- Environment Variable: `FILEBIN_BASEURL`
- Command Line Argument: `--baseurl`
- Default: `https://filebin.net`

The base URL to use, which impacts URLs that are presented to the user for files and bins. It needs to point to the hostname of the filebin instance.

---

**Contact**
- Environment Variable: `FILEBIN_CONTACT`
- Command Line Argument: `--contact`
- Default: (required)

The e-mail address to show on the website on the contact page.

---

**Expiration**
- Environment Variable: `FILEBIN_EXPIRATION`
- Command Line Argument: `--expiration`
- Default: `604800`

Bin expiration time in seconds since the last bin update. Bins will be inaccessible after this time, and files will be removed by the lurker.

---

**Temporary Directory**
- Environment Variable: `FILEBIN_TMPDIR`
- Command Line Argument: `--tmpdir`
- Default: `/tmp`

Directory for temporary files for upload and download.

---

**Temporary Directory Capacity Threshold**
- Environment Variable: `FILEBIN_TMPDIR_CAPACITY_THRESHOLD`
- Command Line Argument: `--tmpdir-capacity-threshold`
- Default: `4.0`

Workspace capacity threshold multiplier. A workspace must have at least this multiplier times the file size available to be selected (e.g., 4.0 requires 4x the file size available).

---

**Manual Approval**
- Environment Variable: `FILEBIN_MANUAL_APPROVAL`
- Command Line Argument: `--manual-approval`
- Default: `false`

If enabled, the administrator needs to manually approve new bins before files and archives can be downloaded. Bin and file operations except downloading are accepted while a bin is pending approval. This is a mechanism added to limit abuse. The API request used to approve a bin is an authenticated `PUT /admin/approve/{bin}`.

---

**Require Verification Cookie**
- Environment Variable: `FILEBIN_REQUIRE_VERIFICATION_COOKIE`
- Command Line Argument: `--require-verification-cookie`
- Default: `false`

If enabled, a warning page will be shown before a user can download files. If the user accepts to continue past the warning page, a cookie will be set to identify that the user understands the risk of downloading files.

---

**Verification Cookie Lifetime**
- Environment Variable: `FILEBIN_VERIFICATION_COOKIE_LIFETIME`
- Command Line Argument: `--verification-cookie-lifetime`
- Default: `365`

Number of days before cookie expiration. See `--require-verification-cookie`.

---

**Expected Cookie Value**
- Environment Variable: `FILEBIN_EXPECTED_COOKIE_VALUE`
- Command Line Argument: `--expected-cookie-value`
- Default: `2024-05-24`

Which cookie value to expect to avoid showing a warning message. See `--require-verification-cookie`.

---

**GeoIP City Database**
- Environment Variable: `FILEBIN_MMDB_CITY`
- Command Line Argument: `--mmdb-city`
- Default: (not set)

The path to an mmdb formatted geoip database like GeoLite2-City.mmdb. This is optional.

---

**GeoIP ASN Database**
- Environment Variable: `FILEBIN_MMDB_ASN`
- Command Line Argument: `--mmdb-asn`
- Default: (not set)

The path to an mmdb formatted geoip database like GeoLite2-ASN.mmdb. This is optional.

---

**Allow Robots**
- Environment Variable: `FILEBIN_ALLOW_ROBOTS`
- Command Line Argument: `--allow-robots`
- Default: `false`

If enabled, the `X-Robots-Tag` response header will allow search engines to index and show Filebin in search results. Otherwise, robots will be instructed to not show files and bins in search results.

---

#### Limits

**Limit File Downloads**
- Environment Variable: `FILEBIN_LIMIT_FILE_DOWNLOADS`
- Command Line Argument: `--limit-file-downloads`
- Default: `0`

Limit the number of downloads per file. 0 means no limit. If the value is 100, then each file can be downloaded 100 times before further downloads are rejected.

---

**Limit Storage**
- Environment Variable: `FILEBIN_LIMIT_STORAGE`
- Command Line Argument: `--limit-storage`
- Default: `0`

Limit the storage capacity that filebin will use. 0 means no limit. If set to "200GB", filebin will allow file uploads until the total storage surpasses 200 GB. New file uploads will be rejected until storage consumption is below 200 GB again.

---

**Reject File Extensions**
- Environment Variable: `FILEBIN_REJECT_FILE_EXTENSIONS`
- Command Line Argument: `--reject-file-extensions`
- Default: (not set)

A whitespace separated list of file extensions that will be rejected. Example: "exe bat dll".

---

#### HTTP Server

**Listen Host**
- Environment Variable: `FILEBIN_LISTEN_HOST`
- Command Line Argument: `--listen-host`
- Default: `127.0.0.1`

Which IP address Filebin will bind to. The default value of 127.0.0.1 is a safe default that does not expose Filebin outside of the host.

---

**Listen Port**
- Environment Variable: `FILEBIN_LISTEN_PORT`
- Command Line Argument: `--listen-port`
- Default: `8080`

Which port Filebin will listen to. The default of 8080 does not require privileged access.

---

**Access Log**
- Environment Variable: `FILEBIN_ACCESS_LOG`
- Command Line Argument: `--access-log`
- Default: `/var/log/filebin/access.log`

Path to a filename for the access log output.

---

**Proxy Headers**
- Environment Variable: `FILEBIN_PROXY_HEADERS`
- Command Line Argument: `--proxy-headers`
- Default: `false`

If enabled, the client IP will be read from the proxy headers provided in the incoming HTTP requests. This should only be enabled if there is an HTTP proxy running in front of Filebin that uses proxy headers to tell Filebin the original client IP address.

---

#### Timeouts

**Read Timeout**
- Environment Variable: `FILEBIN_READ_TIMEOUT`
- Command Line Argument: `--read-timeout`
- Default: `1h`

Read timeout for the HTTP server. File uploads need to complete within this timeout before they are terminated.

---

**Read Header Timeout**
- Environment Variable: `FILEBIN_READ_HEADER_TIMEOUT`
- Command Line Argument: `--read-header-timeout`
- Default: `2s`

Read header timeout for the HTTP server.

---

**Write Timeout**
- Environment Variable: `FILEBIN_WRITE_TIMEOUT`
- Command Line Argument: `--write-timeout`
- Default: `1h`

Write timeout for the HTTP server.

---

**Idle Timeout**
- Environment Variable: `FILEBIN_IDLE_TIMEOUT`
- Command Line Argument: `--idle-timeout`
- Default: `30s`

Idle timeout for the HTTP server.

---

#### Database

**Database Host**
- Environment Variable: `FILEBIN_DATABASE_HOST`
- Command Line Argument: `--db-host`
- Default: (required)

Which PostgreSQL host to connect to. This can be an IP address or a hostname.

---

**Database Port**
- Environment Variable: `FILEBIN_DATABASE_PORT`
- Command Line Argument: `--db-port`
- Default: `5432`

The port to use when connecting to the PostgreSQL database.

---

**Database Name**
- Environment Variable: `FILEBIN_DATABASE_NAME`
- Command Line Argument: `--db-name`
- Default: (required)

The name of the PostgreSQL database to use.

---

**Database Username**
- Environment Variable: `FILEBIN_DATABASE_USERNAME`
- Command Line Argument: `--db-username`
- Default: (required)

The username to use when authenticating to the PostgreSQL database.

---

**Database Password**
- Environment Variable: `FILEBIN_DATABASE_PASSWORD`
- Command Line Argument: `--db-password`
- Default: (required)

The password to use when authenticating to the PostgreSQL database.

---

**Max Open Connections**
- Environment Variable: `FILEBIN_DATABASE_MAX_OPEN_CONNS`
- Command Line Argument: `--db-max-open-conns`
- Default: `25`

Maximum number of open database connections.

---

**Max Idle Connections**
- Environment Variable: `FILEBIN_DATABASE_MAX_IDLE_CONNS`
- Command Line Argument: `--db-max-idle-conns`
- Default: `25`

Maximum number of idle database connections.

---

**Connection Max Lifetime**
- Environment Variable: `FILEBIN_DATABASE_CONN_MAX_LIFETIME`
- Command Line Argument: `--db-conn-max-lifetime`
- Default: `5m`

Maximum time a database connection may be reused before it is closed and replaced. The value is specified using Go duration format, examples: `5m`, `10m`, `1h`.

---

**Connection Max Idle Time**
- Environment Variable: `FILEBIN_DATABASE_CONN_MAX_IDLE_TIME`
- Command Line Argument: `--db-conn-max-idle-time`
- Default: `1m`

Maximum time a database connection may sit idle before it is closed. The value is specified using Go duration format, examples: `1m`, `5m`, `10m`.

---

#### S3 Storage

**S3 Endpoint**
- Environment Variable: `FILEBIN_S3_ENDPOINT`
- Command Line Argument: `--s3-endpoint`
- Default: (required)

The S3 endpoint to connect to. This can be the hostname or IP address. When self hosting S3 on a non-standard port, the port can be specified using `hostname:port`.

---

**S3 Bucket**
- Environment Variable: `FILEBIN_S3_BUCKET`
- Command Line Argument: `--s3-bucket`
- Default: (required)

The name of the bucket in S3 where files will be stored.

---

**S3 Region**
- Environment Variable: `FILEBIN_S3_REGION`
- Command Line Argument: `--s3-region`
- Default: (required)

The S3 region where the bucket lives.

---

**S3 Access Key**
- Environment Variable: `FILEBIN_S3_ACCESS_KEY`
- Command Line Argument: `--s3-access-key`
- Default: (required)

The access key to use when connecting to the S3 bucket where files will be stored.

---

**S3 Secret Key**
- Environment Variable: `FILEBIN_S3_SECRET_KEY`
- Command Line Argument: `--s3-secret-key`
- Default: (required)

The secret key to use when connecting to the S3 bucket where files will be stored.

---

**S3 Secure**
- Environment Variable: `FILEBIN_S3_SECURE`
- Command Line Argument: `--s3-secure`
- Default: `true`

Whether or not Filebin will require the connection to S3 to be TLS encrypted using https. If set to false, Filebin will attempt connecting to S3 using plain http.

---

**S3 URL TTL**
- Environment Variable: `FILEBIN_S3_URL_TTL`
- Command Line Argument: `--s3-url-ttl`
- Default: `1m`

When a Filebin user downloads a file, that is done using a presigned URL that contains a token with limited time to live. The default allows presigned URLs to be used for 1 minute before they expire. The value is specified using the time unit, examples: `30s`, `5m`, `2h`.

---

**S3 Timeout**
- Environment Variable: `FILEBIN_S3_TIMEOUT`
- Command Line Argument: `--s3-timeout`
- Default: `30s`

Timeout for quick S3 operations (delete, head, stat).

---

**S3 Transfer Timeout**
- Environment Variable: `FILEBIN_S3_TRANSFER_TIMEOUT`
- Command Line Argument: `--s3-transfer-timeout`
- Default: `10m`

Timeout for S3 data transfers (put, get, copy).

---

#### Lurker (Background Jobs)

**Lurker Interval**
- Environment Variable: `FILEBIN_LURKER_INTERVAL`
- Command Line Argument: `--lurker-interval`
- Default: `300`

The lurker is a batch job that runs automatically and in the background to delete expired bins and remove old log entries from the database. This specifies the time in seconds for the lurker to sleep between each execution.

---

**Lurker Throttle**
- Environment Variable: `FILEBIN_LURKER_THROTTLE`
- Command Line Argument: `--lurker-throttle`
- Default: `250`

Milliseconds to wait between S3 deletions to throttle the deletion rate.

---

**Log Retention**
- Environment Variable: `FILEBIN_LOG_RETENTION`
- Command Line Argument: `--log-retention`
- Default: `7`

The number of days to keep transaction log entries in the PostgreSQL database before they are removed by the lurker.

---

#### Admin Authentication

**Admin Username**
- Environment Variable: `FILEBIN_ADMIN_USERNAME`
- Command Line Argument: `--admin-username`
- Default: (not set)

Username to require for access to the /admin endpoint. If the username is not set, then the admin endpoint will not be available.

---

**Admin Password**
- Environment Variable: `FILEBIN_ADMIN_PASSWORD`
- Command Line Argument: `--admin-password`
- Default: (not set)

Password to require for access to the /admin endpoint. If the password is not set, then the admin endpoint will not be available. Make sure to keep this password a secret.

---

#### Metrics

**Enable Metrics**
- Environment Variable: `FILEBIN_METRICS`
- Command Line Argument: `--metrics`
- Default: `false`

Enables the `/metrics` endpoint. If this is not set, the endpoint will not return any metrics.

---

**Metrics Username**
- Environment Variable: `FILEBIN_METRICS_USERNAME`
- Command Line Argument: `--metrics-username`
- Default: (not set)

The username used for authentication to the `/metrics` endpoint for Prometheus metrics.

---

**Metrics Password**
- Environment Variable: `FILEBIN_METRICS_PASSWORD`
- Command Line Argument: `--metrics-password`
- Default: (not set)

The password used for authentication to the `/metrics` endpoint for Prometheus metrics.

---

**Metrics Auth**
- Environment Variable: `FILEBIN_METRICS_AUTH`
- Command Line Argument: `--metrics-auth`
- Default: (not set)

Enables authentication. Currently only basic auth is supported. If set to `basic`, basic auth will be required. If not set, the endpoint is open to the world.

---

**Metrics ID**
- Environment Variable: `FILEBIN_METRICS_ID`
- Command Line Argument: `--metrics-id`
- Default: `$HOSTNAME`

The string used as the identification of the filebin instance in the Prometheus metrics. By default, this is the `$HOSTNAME` environment variable.

---

**Metrics Proxy URL**
- Environment Variable: `FILEBIN_METRICS_PROXY_URL`
- Command Line Argument: `--metrics-proxy-url`
- Default: (not set)

When set, Filebin will fetch the content from the URL specified and merge it with its own output on the `/metrics` endpoint. This can be useful when running another Prometheus exporter in the same operating system instance, for example to capture system metrics.

---

#### Slack Integration

**Slack Secret**
- Environment Variable: `FILEBIN_SLACK_SECRET`
- Command Line Argument: `--slack-secret`
- Default: (not set)

The secret that Slack will need to use when connecting to the http api. If this secret is not set, the Slack integration will be disabled.

---

**Slack Domain**
- Environment Variable: `FILEBIN_SLACK_DOMAIN`
- Command Line Argument: `--slack-domain`
- Default: (not set)

Limits which Slack domain is allowed to access the http api. Other domains will be rejected.

---

**Slack Channel**
- Environment Variable: `FILEBIN_SLACK_CHANNEL`
- Command Line Argument: `--slack-channel`
- Default: (not set)

If Filebin is set to require manual approval of new bins, then this approval can be given using the user interface, the http api directly or via Slack (using the http api). The http endpoint `/integration/slack` can be accessed using a webhook from Slack. This parameter limits which Slack channel is allowed to access the http api. Requests from other channels will be rejected.

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
