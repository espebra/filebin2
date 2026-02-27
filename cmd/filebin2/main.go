package main

import (
	"flag"
	"fmt"
	"log/slog"
	_ "net/http/pprof"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/espebra/filebin2/internal/dbl"
	"github.com/espebra/filebin2/internal/ds"
	"github.com/espebra/filebin2/internal/geoip"
	"github.com/espebra/filebin2/internal/lurker"
	"github.com/espebra/filebin2/internal/s3"
	"github.com/espebra/filebin2/internal/web"
	"github.com/espebra/filebin2/internal/workspace"

	"github.com/dustin/go-humanize"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Version information (set via ldflags at build time)
var (
	version = "dev"
	commit  = "unknown"
)

var (
	// Various
	contactFlag             = flag.String("contact", "", "The contact information, such as an email address, that will be shown on the website for people that want to get in touch with the service owner.")
	expirationFlag          = flag.Int("expiration", 604800, "Bin expiration time in seconds since the last bin update")
	tmpdirFlag              = flag.String("tmpdir", os.TempDir(), "Comma-separated list of directories for temporary files. Multiple directories will be benchmarked at startup, and the fastest one with sufficient free space will be used for each upload.")
	tmpdirThresholdFlag     = flag.Float64("tmpdir-capacity-threshold", 4.0, "Workspace capacity threshold multiplier. A workspace must have at least this multiplier times the file size available to be selected (e.g., 4.0 requires 4x the file size available).")
	baseURLFlag             = flag.String("baseurl", "https://filebin.net", "The base URL to use. Required for self-hosted instances.")
	requireApprovalFlag     = flag.Bool("manual-approval", false, "Require manual admin approval of new bins before files can be downloaded.")
	requireCookieFlag       = flag.Bool("require-verification-cookie", false, "Require cookie before allowing a download to happen.")
	cookieLifetimeFlag      = flag.Int("verification-cookie-lifetime", 365, "Number of days before cookie expiration.")
	expectedCookieValueFlag = flag.String("expected-cookie-value", "2024-05-24", "Which cookie value to expect to avoid showing a warning message.")
	mmdbCityPathFlag        = flag.String("mmdb-city", "", "The path to an mmdb formatted geoip database like GeoLite2-City.mmdb.")
	mmdbASNPathFlag         = flag.String("mmdb-asn", "", "The path to an mmdb formatted geoip database like GeoLite2-ASN.mmdb.")
	allowRobotsFlag         = flag.Bool("allow-robots", false, "Allow robots to crawl and index the site (using X-Robots-Tag response header).")

	// Limits
	limitFileDownloadsFlag = flag.Uint64("limit-file-downloads", 0, "Limit the number of downloads per file. 0 disables this limit.")
	limitStorageFlag       = flag.String("limit-storage", "0", "Limit the storage capacity to use (examples: 100MB, 20GB, 2TB). 0 disables this limit.")
	rejectFileExtensions   = flag.String("reject-file-extensions", "", "A whitespace separated list of file extensions that will be rejected")

	// HTTP
	listenHostFlag   = flag.String("listen-host", "127.0.0.1", "Listen host")
	listenPortFlag   = flag.Int("listen-port", 8080, "Listen port")
	accessLogFlag    = flag.String("access-log", "/var/log/filebin/access.log", "Path for access.log output")
	proxyHeadersFlag = flag.Bool("proxy-headers", false, "Read client request information from proxy headers")

	// Timeouts
	readTimeoutFlag       = flag.Duration("read-timeout", 1*time.Hour, "Read timeout for the HTTP server")
	readHeaderTimeoutFlag = flag.Duration("read-header-timeout", 2*time.Second, "Read header timeout for the HTTP server")
	writeTimeoutFlag      = flag.Duration("write-timeout", 1*time.Hour, "Write timeout for the HTTP server")
	idleTimeoutFlag       = flag.Duration("idle-timeout", 30*time.Second, "Idle timeout for the HTTP server")

	// Database
	dbHostFlag            = flag.String("db-host", "", "Database host")
	dbPortFlag            = flag.String("db-port", "", "Database port")
	dbNameFlag            = flag.String("db-name", "", "Name of the database")
	dbUsernameFlag        = flag.String("db-username", "", "Database username")
	dbPasswordFlag        = flag.String("db-password", "", "Database password")
	dbMaxOpenConnsFlag    = flag.Int("db-max-open-conns", 25, "Maximum number of open database connections")
	dbMaxIdleConnsFlag    = flag.Int("db-max-idle-conns", 25, "Maximum number of idle database connections")
	dbConnMaxLifetimeFlag = flag.Duration("db-conn-max-lifetime", 5*time.Minute, "Maximum time a database connection may be reused")
	dbConnMaxIdleTimeFlag = flag.Duration("db-conn-max-idle-time", 1*time.Minute, "Maximum time a database connection may be idle before being closed")

	// S3
	s3EndpointFlag             = flag.String("s3-endpoint", "", "S3 endpoint")
	s3BucketFlag               = flag.String("s3-bucket", "", "S3 bucket")
	s3RegionFlag               = flag.String("s3-region", "", "S3 region")
	s3AccessKeyFlag            = flag.String("s3-access-key", "", "S3 access key")
	s3SecretKeyFlag            = flag.String("s3-secret-key", "", "S3 secret key")
	s3SecureFlag               = flag.Bool("s3-secure", true, "Use TLS when connecting to S3")
	s3UrlTtlFlag               = flag.String("s3-url-ttl", "1m", "The time to live for presigned S3 URLs, for example 30s or 5m")
	s3TimeoutFlag              = flag.String("s3-timeout", "30s", "Timeout for quick S3 operations (delete, head, stat)")
	s3TransferTimeoutFlag      = flag.String("s3-transfer-timeout", "10m", "Timeout for S3 data transfers (put, get, copy)")
	s3MultipartPartSizeFlag    = flag.String("s3-multipart-part-size", "64MB", "Multipart upload part size (e.g. 5MB, 64MB, 128MB). Files larger than this use multipart upload.")
	s3MultipartConcurrencyFlag = flag.Int("s3-multipart-concurrency", 3, "Number of concurrent part uploads for multipart uploads")

	// Lurker
	lurkerIntervalFlag = flag.Int("lurker-interval", 300, "Lurker interval is the delay to sleep between each run in seconds")
	lurkerThrottleFlag = flag.Int("lurker-throttle", 250, "Milliseconds to sleep between each S3 deletion")
	logRetentionFlag   = flag.Uint64("log-retention", 7, "The number of days to keep log entries before removed by the lurker.")

	// Auth
	adminUsernameFlag   = flag.String("admin-username", "", "Admin username")
	adminPasswordFlag   = flag.String("admin-password", "", "Admin password")
	metricsUsernameFlag = flag.String("metrics-username", "", "Metrics username")
	metricsPasswordFlag = flag.String("metrics-password", "", "Metrics password")
	metricsFlag         = flag.Bool("metrics", false, "Enable the metrics endpoint")
	metricsAuthFlag     = flag.String("metrics-auth", "", "Set the auth type for the metrics endpoint")
	metricsIdFlag       = flag.String("metrics-id", "", "Metrics instance identification")
	metricsProxyURLFlag = flag.String("metrics-proxy-url", "", "URL to another Prometheus exporter that we should proxy")

	// Slack integration
	slackSecretFlag  = flag.String("slack-secret", "", "Slack secret (currently used to approve new bins via Slack if manual approval is enabled)")
	slackDomainFlag  = flag.String("slack-domain", "", "Slack domain")
	slackChannelFlag = flag.String("slack-channel", "", "Slack channel")

	// Version
	versionFlag = flag.Bool("version", false, "Print version information and exit")

	// Logging
	logFormatFlag = flag.String("log-format", "text", "Log output format: text or json")
	logLevelFlag  = flag.String("log-level", "info", "Log level: debug, info, warn, or error")
)

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Printf("filebin %s (commit: %s)\n", version, commit)
		os.Exit(0)
	}

	// Environment variable overrides (FILEBIN_ prefix)
	// All configuration can be set via environment variables.
	// Command line flags take precedence over environment variables.

	// Logging (handle early so we can use structured logging for the rest of setup)
	if v := os.Getenv("FILEBIN_LOG_FORMAT"); v != "" && *logFormatFlag == "text" {
		*logFormatFlag = v
	}
	if v := os.Getenv("FILEBIN_LOG_LEVEL"); v != "" && *logLevelFlag == "info" {
		*logLevelFlag = v
	}

	// Configure structured logging
	configureLogger(*logFormatFlag, *logLevelFlag)

	slog.Info("starting filebin", "version", version, "commit", commit)

	// Various
	if *contactFlag == "" {
		*contactFlag = os.Getenv("FILEBIN_CONTACT")
	}
	if v := os.Getenv("FILEBIN_EXPIRATION"); v != "" && *expirationFlag == 604800 {
		if i, err := strconv.Atoi(v); err == nil {
			*expirationFlag = i
		}
	}
	if v := os.Getenv("FILEBIN_TMPDIR"); v != "" && *tmpdirFlag == os.TempDir() {
		*tmpdirFlag = v
	}
	if v := os.Getenv("FILEBIN_TMPDIR_CAPACITY_THRESHOLD"); v != "" && *tmpdirThresholdFlag == 4.0 {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			*tmpdirThresholdFlag = f
		}
	}
	if v := os.Getenv("FILEBIN_BASEURL"); v != "" && *baseURLFlag == "https://filebin.net" {
		*baseURLFlag = v
	}
	if v := os.Getenv("FILEBIN_MANUAL_APPROVAL"); v != "" {
		*requireApprovalFlag = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("FILEBIN_REQUIRE_VERIFICATION_COOKIE"); v != "" {
		*requireCookieFlag = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("FILEBIN_VERIFICATION_COOKIE_LIFETIME"); v != "" && *cookieLifetimeFlag == 365 {
		if i, err := strconv.Atoi(v); err == nil {
			*cookieLifetimeFlag = i
		}
	}
	if v := os.Getenv("FILEBIN_EXPECTED_COOKIE_VALUE"); v != "" && *expectedCookieValueFlag == "2024-05-24" {
		*expectedCookieValueFlag = v
	}
	if *mmdbCityPathFlag == "" {
		*mmdbCityPathFlag = os.Getenv("FILEBIN_MMDB_CITY")
	}
	if *mmdbASNPathFlag == "" {
		*mmdbASNPathFlag = os.Getenv("FILEBIN_MMDB_ASN")
	}
	if v := os.Getenv("FILEBIN_ALLOW_ROBOTS"); v != "" {
		*allowRobotsFlag = v == "true" || v == "1" || v == "yes"
	}

	// Limits
	if v := os.Getenv("FILEBIN_LIMIT_FILE_DOWNLOADS"); v != "" && *limitFileDownloadsFlag == 0 {
		if i, err := strconv.ParseUint(v, 10, 64); err == nil {
			*limitFileDownloadsFlag = i
		}
	}
	if v := os.Getenv("FILEBIN_LIMIT_STORAGE"); v != "" && *limitStorageFlag == "0" {
		*limitStorageFlag = v
	}
	if *rejectFileExtensions == "" {
		*rejectFileExtensions = os.Getenv("FILEBIN_REJECT_FILE_EXTENSIONS")
	}

	// HTTP
	if v := os.Getenv("FILEBIN_LISTEN_HOST"); v != "" && *listenHostFlag == "127.0.0.1" {
		*listenHostFlag = v
	}
	if v := os.Getenv("FILEBIN_LISTEN_PORT"); v != "" && *listenPortFlag == 8080 {
		if i, err := strconv.Atoi(v); err == nil {
			*listenPortFlag = i
		}
	}
	if v := os.Getenv("FILEBIN_ACCESS_LOG"); v != "" && *accessLogFlag == "/var/log/filebin/access.log" {
		*accessLogFlag = v
	}
	if v := os.Getenv("FILEBIN_PROXY_HEADERS"); v != "" {
		*proxyHeadersFlag = v == "true" || v == "1" || v == "yes"
	}

	// Timeouts
	if v := os.Getenv("FILEBIN_READ_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			*readTimeoutFlag = d
		}
	}
	if v := os.Getenv("FILEBIN_READ_HEADER_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			*readHeaderTimeoutFlag = d
		}
	}
	if v := os.Getenv("FILEBIN_WRITE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			*writeTimeoutFlag = d
		}
	}
	if v := os.Getenv("FILEBIN_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			*idleTimeoutFlag = d
		}
	}

	// Database
	if *dbHostFlag == "" {
		*dbHostFlag = os.Getenv("FILEBIN_DATABASE_HOST")
	}
	if *dbPortFlag == "" {
		*dbPortFlag = os.Getenv("FILEBIN_DATABASE_PORT")
	}
	if *dbNameFlag == "" {
		*dbNameFlag = os.Getenv("FILEBIN_DATABASE_NAME")
	}
	if *dbUsernameFlag == "" {
		*dbUsernameFlag = os.Getenv("FILEBIN_DATABASE_USERNAME")
	}
	if *dbPasswordFlag == "" {
		*dbPasswordFlag = os.Getenv("FILEBIN_DATABASE_PASSWORD")
	}
	if v := os.Getenv("FILEBIN_DATABASE_MAX_OPEN_CONNS"); v != "" && *dbMaxOpenConnsFlag == 25 {
		if i, err := strconv.Atoi(v); err == nil {
			*dbMaxOpenConnsFlag = i
		}
	}
	if v := os.Getenv("FILEBIN_DATABASE_MAX_IDLE_CONNS"); v != "" && *dbMaxIdleConnsFlag == 25 {
		if i, err := strconv.Atoi(v); err == nil {
			*dbMaxIdleConnsFlag = i
		}
	}
	if v := os.Getenv("FILEBIN_DATABASE_CONN_MAX_LIFETIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			*dbConnMaxLifetimeFlag = d
		}
	}
	if v := os.Getenv("FILEBIN_DATABASE_CONN_MAX_IDLE_TIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			*dbConnMaxIdleTimeFlag = d
		}
	}

	// S3
	if *s3EndpointFlag == "" {
		*s3EndpointFlag = os.Getenv("FILEBIN_S3_ENDPOINT")
	}
	if *s3BucketFlag == "" {
		*s3BucketFlag = os.Getenv("FILEBIN_S3_BUCKET")
	}
	if *s3RegionFlag == "" {
		*s3RegionFlag = os.Getenv("FILEBIN_S3_REGION")
	}
	if *s3AccessKeyFlag == "" {
		*s3AccessKeyFlag = os.Getenv("FILEBIN_S3_ACCESS_KEY")
	}
	if *s3SecretKeyFlag == "" {
		*s3SecretKeyFlag = os.Getenv("FILEBIN_S3_SECRET_KEY")
	}
	if v := os.Getenv("FILEBIN_S3_SECURE"); v != "" {
		*s3SecureFlag = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("FILEBIN_S3_URL_TTL"); v != "" && *s3UrlTtlFlag == "1m" {
		*s3UrlTtlFlag = v
	}
	if v := os.Getenv("FILEBIN_S3_TIMEOUT"); v != "" && *s3TimeoutFlag == "30s" {
		*s3TimeoutFlag = v
	}
	if v := os.Getenv("FILEBIN_S3_TRANSFER_TIMEOUT"); v != "" && *s3TransferTimeoutFlag == "10m" {
		*s3TransferTimeoutFlag = v
	}
	if v := os.Getenv("FILEBIN_S3_MULTIPART_PART_SIZE"); v != "" && *s3MultipartPartSizeFlag == "64MB" {
		*s3MultipartPartSizeFlag = v
	}
	if v := os.Getenv("FILEBIN_S3_MULTIPART_CONCURRENCY"); v != "" && *s3MultipartConcurrencyFlag == 3 {
		if i, err := strconv.Atoi(v); err == nil {
			*s3MultipartConcurrencyFlag = i
		}
	}

	// Lurker
	if v := os.Getenv("FILEBIN_LURKER_INTERVAL"); v != "" && *lurkerIntervalFlag == 300 {
		if i, err := strconv.Atoi(v); err == nil {
			*lurkerIntervalFlag = i
		}
	}
	if v := os.Getenv("FILEBIN_LURKER_THROTTLE"); v != "" && *lurkerThrottleFlag == 250 {
		if i, err := strconv.Atoi(v); err == nil {
			*lurkerThrottleFlag = i
		}
	}
	if v := os.Getenv("FILEBIN_LOG_RETENTION"); v != "" && *logRetentionFlag == 7 {
		if i, err := strconv.ParseUint(v, 10, 64); err == nil {
			*logRetentionFlag = i
		}
	}

	// Auth
	if *adminUsernameFlag == "" {
		*adminUsernameFlag = os.Getenv("FILEBIN_ADMIN_USERNAME")
	}
	if *adminPasswordFlag == "" {
		*adminPasswordFlag = os.Getenv("FILEBIN_ADMIN_PASSWORD")
	}
	if *metricsUsernameFlag == "" {
		*metricsUsernameFlag = os.Getenv("FILEBIN_METRICS_USERNAME")
	}
	if *metricsPasswordFlag == "" {
		*metricsPasswordFlag = os.Getenv("FILEBIN_METRICS_PASSWORD")
	}
	if v := os.Getenv("FILEBIN_METRICS"); v != "" {
		*metricsFlag = v == "true" || v == "1" || v == "yes"
	}
	if *metricsAuthFlag == "" {
		*metricsAuthFlag = os.Getenv("FILEBIN_METRICS_AUTH")
	}
	if *metricsIdFlag == "" {
		*metricsIdFlag = os.Getenv("FILEBIN_METRICS_ID")
	}
	if *metricsIdFlag == "" {
		*metricsIdFlag = os.Getenv("HOSTNAME")
	}
	if *metricsProxyURLFlag == "" {
		*metricsProxyURLFlag = os.Getenv("FILEBIN_METRICS_PROXY_URL")
	}

	// Slack integration
	if *slackSecretFlag == "" {
		*slackSecretFlag = os.Getenv("FILEBIN_SLACK_SECRET")
	}
	if *slackDomainFlag == "" {
		*slackDomainFlag = os.Getenv("FILEBIN_SLACK_DOMAIN")
	}
	if *slackChannelFlag == "" {
		*slackChannelFlag = os.Getenv("FILEBIN_SLACK_CHANNEL")
	}

	// Validate metrics proxy URL if set
	if *metricsProxyURLFlag != "" {
		_, err := url.Parse(*metricsProxyURLFlag)
		if err != nil {
			slog.Error("unable to parse --metrics-proxy-url", "error", err)
			os.Exit(2)
		}
	}

	// Contact information is required
	if *contactFlag == "" {
		slog.Error("contact information must be specified using --contact or FILEBIN_CONTACT")
		os.Exit(2)
	}

	// mmdb path
	geodb, err := geoip.Init(*mmdbASNPathFlag, *mmdbCityPathFlag)
	if err != nil {
		slog.Error("unable to load geoip database", "error", err)
		os.Exit(2)
	}

	// Set database port to 5432 if not set or invalid
	dbport, err := strconv.Atoi(*dbPortFlag)
	if err != nil {
		dbport = 5432
	}

	daoconn, err := dbl.Init(dbl.DBConfig{
		Host:            *dbHostFlag,
		Port:            dbport,
		Name:            *dbNameFlag,
		Username:        *dbUsernameFlag,
		Password:        *dbPasswordFlag,
		MaxOpenConns:    *dbMaxOpenConnsFlag,
		MaxIdleConns:    *dbMaxIdleConnsFlag,
		ConnMaxLifetime: *dbConnMaxLifetimeFlag,
		ConnMaxIdleTime: *dbConnMaxIdleTimeFlag,
	})
	if err != nil {
		slog.Error("unable to connect to the database", "error", err)
		os.Exit(2)
	}

	s3UrlTtl, err := time.ParseDuration(*s3UrlTtlFlag)
	if err != nil {
		slog.Error("unable to parse --s3-url-ttl", "error", err)
		os.Exit(2)
	}
	slog.Info("configured presigned S3 URL TTL", "ttl_seconds", s3UrlTtl.Seconds())

	s3Timeout, err := time.ParseDuration(*s3TimeoutFlag)
	if err != nil {
		slog.Error("unable to parse --s3-timeout", "error", err)
		os.Exit(2)
	}

	s3TransferTimeout, err := time.ParseDuration(*s3TransferTimeoutFlag)
	if err != nil {
		slog.Error("unable to parse --s3-transfer-timeout", "error", err)
		os.Exit(2)
	}

	filter := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	for _, v := range strings.Fields(*rejectFileExtensions) {
		if !filter.Match([]byte(v)) {
			slog.Error("extension specified by --reject-file-extensions contains illegal characters", "extension", v)
			os.Exit(2)
		}
		slog.Info("rejecting file extension", "extension", v)
	}

	s3MultipartPartSize, err := humanize.ParseBytes(*s3MultipartPartSizeFlag)
	if err != nil {
		slog.Error("unable to parse --s3-multipart-part-size", "error", err)
		os.Exit(2)
	}
	slog.Info("configured S3 multipart upload", "part_size", humanize.Bytes(s3MultipartPartSize), "concurrency", *s3MultipartConcurrencyFlag)

	s3conn, err := s3.Init(s3.Config{
		Endpoint:             *s3EndpointFlag,
		Bucket:               *s3BucketFlag,
		Region:               *s3RegionFlag,
		AccessKey:            *s3AccessKeyFlag,
		SecretKey:            *s3SecretKeyFlag,
		Secure:               *s3SecureFlag,
		PresignExpiry:        s3UrlTtl,
		Timeout:              s3Timeout,
		TransferTimeout:      s3TransferTimeout,
		MultipartPartSize:    int64(s3MultipartPartSize),
		MultipartConcurrency: *s3MultipartConcurrencyFlag,
	})
	if err != nil {
		slog.Error("unable to initialize S3 connection", "error", err)
		os.Exit(2)
	}

	// Initialize workspace manager
	wm, err := workspace.NewManager(*tmpdirFlag, *tmpdirThresholdFlag)
	if err != nil {
		slog.Error("unable to initialize workspace manager", "error", err)
		os.Exit(2)
	}
	slog.Info("workspace capacity threshold configured", "threshold", fmt.Sprintf("%.1fx file size", *tmpdirThresholdFlag))

	// Clean up stale temporary files from previous runs
	wm.CleanStaleFiles(24 * time.Hour)

	// Create and start the lurker process
	l := lurker.New(&daoconn, &s3conn, wm)
	l.Init(*lurkerIntervalFlag, *lurkerThrottleFlag, *logRetentionFlag)
	l.Run()

	u, err := url.Parse(*baseURLFlag)
	if err != nil {
		slog.Error("unable to parse the baseurl parameter", "baseurl", *baseURLFlag, "error", err)
		os.Exit(2)
	}

	config := &ds.Config{
		Version:              version,
		AdminPassword:        *adminPasswordFlag,
		AdminUsername:        *adminUsernameFlag,
		Contact:              *contactFlag,
		MetricsPassword:      *metricsPasswordFlag,
		MetricsUsername:      *metricsUsernameFlag,
		Metrics:              *metricsFlag,
		MetricsAuth:          *metricsAuthFlag,
		MetricsProxyURL:      *metricsProxyURLFlag,
		AllowRobots:          *allowRobotsFlag,
		BaseUrl:              *u,
		Expiration:           *expirationFlag,
		HttpHost:             *listenHostFlag,
		HttpAccessLog:        *accessLogFlag,
		HttpPort:             *listenPortFlag,
		HttpProxyHeaders:     *proxyHeadersFlag,
		IdleTimeout:          *idleTimeoutFlag,
		LimitFileDownloads:   *limitFileDownloadsFlag,
		ReadHeaderTimeout:    *readHeaderTimeoutFlag,
		ReadTimeout:          *readTimeoutFlag,
		RequireApproval:      *requireApprovalFlag,
		RequireCookie:        *requireCookieFlag,
		CookieLifetime:       *cookieLifetimeFlag,
		ExpectedCookieValue:  *expectedCookieValueFlag,
		RejectFileExtensions: strings.Fields(*rejectFileExtensions),
		SlackSecret:          *slackSecretFlag,
		SlackDomain:          *slackDomainFlag,
		SlackChannel:         *slackChannelFlag,
		Tmpdir:               *tmpdirFlag,
		WriteTimeout:         *writeTimeoutFlag,
	}

	config.LimitStorageBytes, err = humanize.ParseBytes(*limitStorageFlag)
	if err != nil {
		slog.Error("unable to parse the --limit-storage parameter", "value", *limitStorageFlag, "error", err)
		os.Exit(2)
	}
	config.LimitStorageReadable = humanize.Bytes(config.LimitStorageBytes)

	// Create Prometheus registry and metrics
	metricsRegistry := prometheus.NewRegistry()
	metricsRegistry.MustRegister(collectors.NewGoCollector())
	metricsRegistry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	metrics := ds.NewMetrics(*metricsIdFlag, metricsRegistry)
	metrics.LimitBytes = config.LimitStorageBytes

	// Wire S3 metrics
	s3conn.SetMetrics(metrics)

	// Wire database metrics
	daoconn.SetMetrics(metrics)

	// Create and initialize HTTP server
	h := web.New(&daoconn, &s3conn, &geodb, wm, config, metrics, metricsRegistry)

	if err := h.Init(); err != nil {
		slog.Error("unable to start the HTTP server", "error", err)
		os.Exit(2)
	}
	slog.Info("uploaded files expiration configured", "expiration_seconds", config.ExpirationDuration.Seconds())

	// Start the http server
	h.Run()
}

// configureLogger sets up the default slog logger based on format and level
func configureLogger(format, level string) {
	opts := &slog.HandlerOptions{
		Level: parseLogLevel(level),
	}

	var handler slog.Handler
	if strings.ToLower(format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
