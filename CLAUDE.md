# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Filebin2 is a web application for convenient file sharing built in Go. It uses PostgreSQL for metadata and S3-compatible storage (MinIO) for file storage.

## Development Commands

### Build
```bash
make linux          # Build for Linux (output: artifacts/filebin2-linux-amd64)
make darwin         # Build for macOS (output: artifacts/filebin2-darwin-amd64)
make                # Build and run tests, used during the integration testing below
```

### Testing
```bash
# Tests are run with podman or docker compose, which is preferred for full integration testing
podman compose -f ci.yml up --abort-on-container-exit
```

The test environment is run with:
- Race detection enabled (`-race`)
- Coverage reporting (generates `artifacts/coverage.html`)
- JUnit XML output for CI (`tests/tests.xml`)
- Sequential execution (`-p 1`)

### Code Formatting
```bash
make fmt            # Format all Go files in main package and subpackages (ds, s3, dbl, geoip)
```

### Local Development Environment
```bash
podman compose up --build   # Start PostgreSQL, MinIO, and filebin2

# Services available at:
# - Filebin2: http://localhost:8080/
# - Admin: http://admin:changeme@localhost:8080/admin
# - MinIO: http://localhost:9000/ (console on :9001)
# - PostgreSQL: localhost:5432
```

## Architecture

### Package Structure

The codebase is organized into distinct layers:

**`dbl/` - Database Layer**
- Provides database abstraction over PostgreSQL
- `DAO` struct with specialized sub-DAOs: `BinDao`, `FileDao`, `MetricsDao`, `TransactionDao`, `ClientDao`
- All database operations go through this layer
- Database schema defined in `dbl/schema.sql`

**`ds/` - Data Structures**
- Domain models: `Bin`, `File`, `Client`, `Transaction`, `Metrics`, `Config`
- Pure data structures with no business logic
- Shared across all layers

**`s3/` - S3 Storage Layer**
- `S3AO` struct handles all S3/MinIO interactions
- Uses minio-go client library
- Manages file uploads, downloads, and presigned URLs
- Tracks bucket metrics

**`geoip/` - GeoIP Lookups**
- Optional MaxMind database integration for IP geolocation
- Provides ASN and city-level location data

**Root package (`main`)** - HTTP handlers and application logic
- `http.go` - Main HTTP struct and router setup using gorilla/mux
- `http_*.go` files - Handlers organized by functionality:
  - `http_bin.go` - Bin operations (create, view, delete)
  - `http_file.go` - File upload/download operations
  - `http_admin.go` - Admin dashboard and operations
  - `http_metrics.go` - Prometheus metrics endpoint
  - `http_integration.go` - Slack integration
- `lurker.go` - Background job that periodically:
  - Deletes expired bins and files
  - Cleans old transaction logs
  - Removes stale client records
- `main.go` - Application entry point and flag parsing

### Data Flow

1. **File Upload**: HTTP handler → DAO (metadata to PostgreSQL) → S3AO (file to S3) → Transaction log
2. **File Download**: HTTP handler → DAO (check approval, generate presigned URL) → S3AO → Redirect to presigned S3 URL
3. **Bin Lifecycle**: Created on first file upload → Updated on subsequent uploads → Expires after inactivity → Lurker deletes files and metadata

### Database Schema

Core tables:
- `bin` - Bins (collections of files) with approval, expiration tracking
- `file` - File metadata with SHA256 checksums, references bin via foreign key
- `transaction` - Audit log of all HTTP operations
- `client` - IP-based client tracking with geolocation and ban capability
- `autonomous_system` - ASN tracking for network-level analysis

### Key Configuration Patterns

The application uses command-line flags extensively (defined in `main.go`). Important flags:
- Database: `--db-host`, `--db-port`, `--db-name`, `--db-username`, `--db-password`
- S3: `--s3-endpoint`, `--s3-bucket`, `--s3-region`, `--s3-access-key`, `--s3-secret-key`, `--s3-secure`
- Features: `--manual-approval`, `--require-verification-cookie`, `--limit-storage`, `--limit-file-downloads`
- Admin: `--admin-username`, `--admin-password`
- Lurker: `--lurker-interval` (seconds between cleanup runs), `--expiration` (bin lifetime in seconds)

Environment variables can be used for sensitive values (e.g., `DATABASE_PASSWORD`, `ADMIN_PASSWORD`).

### Embedded Assets

The application embeds static files and templates at build time:
```go
//go:embed templates
templateBox embed.FS

//go:embed static
staticBox embed.FS
```

### Middleware Chain

HTTP handlers use middleware wrappers:
- `log()` - Transaction logging
- `auth()` - Basic auth for admin endpoints
- `clientLookup()` - IP tracking and geolocation

## Important Behaviors

- **Vendored dependencies**: Build commands use `-mod=vendor`
- **Manual approval**: If enabled, bins must be approved via `/admin/approve/{bin}` before downloads are allowed
- **Lurker process**: Runs in background on an interval to clean up expired data
- **Presigned URLs**: File downloads use time-limited presigned S3 URLs (default: 1 minute TTL)
- **Bin expiration**: Bins expire after inactivity period (default: 7 days)
- **Sequential testing**: Tests run with `-p 1` to avoid race conditions in database operations
