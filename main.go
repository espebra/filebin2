package main

import (
	"embed"
	"flag"
	"fmt"
	"math/rand"
	_ "net/http/pprof"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	//"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/geoip"
	"github.com/espebra/filebin2/s3"
	"github.com/espebra/filebin2/workspace"

	"github.com/dustin/go-humanize"
	"github.com/prometheus/client_golang/prometheus"
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
	//enableBanningFlag = flag.Bool("enable-banning", false, "Enable banning. This will allow anyone to ban client IP addresses that upload files to filebin.")
	mmdbCityPathFlag = flag.String("mmdb-city", "", "The path to an mmdb formatted geoip database like GeoLite2-City.mmdb.")
	mmdbASNPathFlag  = flag.String("mmdb-asn", "", "The path to an mmdb formatted geoip database like GeoLite2-ASN.mmdb.")
	allowRobotsFlag  = flag.Bool("allow-robots", false, "Allow robots to crawl and index the site (using X-Robots-Tag response header).")

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
	dbHostFlag     = flag.String("db-host", os.Getenv("DATABASE_HOST"), "Database host")
	dbPortFlag     = flag.String("db-port", os.Getenv("DATABASE_PORT"), "Database port")
	dbNameFlag     = flag.String("db-name", os.Getenv("DATABASE_NAME"), "Name of the database")
	dbUsernameFlag = flag.String("db-username", "", "Database username")
	dbPasswordFlag = flag.String("db-password", "", "Database password")

	// S3
	s3EndpointFlag  = flag.String("s3-endpoint", os.Getenv("S3_ENDPOINT"), "S3 endpoint")
	s3BucketFlag    = flag.String("s3-bucket", os.Getenv("S3_BUCKET"), "S3 bucket")
	s3RegionFlag    = flag.String("s3-region", os.Getenv("S3_REGION"), "S3 region")
	s3AccessKeyFlag = flag.String("s3-access-key", "", "S3 access key")
	s3SecretKeyFlag = flag.String("s3-secret-key", "", "S3 secret key")
	s3TraceFlag     = flag.Bool("s3-trace", false, "Enable S3 HTTP tracing for debugging")
	s3SecureFlag    = flag.Bool("s3-secure", true, "Use TLS when connecting to S3")
	s3UrlTtlFlag    = flag.String("s3-url-ttl", "1m", "The time to live for presigned S3 URLs, for example 30s or 5m")

	// Lurker
	lurkerIntervalFlag = flag.Int("lurker-interval", 300, "Lurker interval is the delay to sleep between each run in seconds")
	logRetentionFlag   = flag.Uint64("log-retention", 7, "The number of days to keep log entries before removed by the lurker.")

	// Auth
	adminUsernameFlag   = flag.String("admin-username", "", "Admin username")
	adminPasswordFlag   = flag.String("admin-password", "", "Admin password")
	metricsUsernameFlag = flag.String("metrics-username", "", "Metrics username")
	metricsPasswordFlag = flag.String("metrics-password", "", "Metrics password")
	metricsFlag         = flag.Bool("metrics", false, "Enable the metrics endpoint")
	metricsAuthFlag     = flag.String("metrics-auth", "", "Set the auth type for the metrics endpoint")
	metricsIdFlag       = flag.String("metrics-id", os.Getenv("METRICS_ID"), "Metrics instance identification")
	metricsProxyURLFlag = flag.String("metrics-proxy-url", "", "URL to another Prometheus exporter that we should proxy")

	// Slack integration
	slackSecretFlag  = flag.String("slack-secret", "", "Slack secret (currently used to approve new bins via Slack if manual approval is enabled)")
	slackDomainFlag  = flag.String("slack-domain", os.Getenv("SLACK_DOMAIN"), "Slack domain")
	slackChannelFlag = flag.String("slack-channel", os.Getenv("SLACK_CHANNEL"), "Slack channel")

	//go:embed templates
	templateBox embed.FS

	//go:embed static
	staticBox embed.FS
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	flag.Parse()

	// Set some default values that should not be exposed by flag.PrintDefaults()
	if *dbUsernameFlag == "" {
		*dbUsernameFlag = os.Getenv("DATABASE_USERNAME")
	}
	if *dbPasswordFlag == "" {
		*dbPasswordFlag = os.Getenv("DATABASE_PASSWORD")
	}
	if *s3AccessKeyFlag == "" {
		*s3AccessKeyFlag = os.Getenv("S3_ACCESS_KEY")
	}
	if *s3SecretKeyFlag == "" {
		*s3SecretKeyFlag = os.Getenv("S3_SECRET_KEY")
	}
	if *adminUsernameFlag == "" {
		*adminUsernameFlag = os.Getenv("ADMIN_USERNAME")
	}
	if *adminPasswordFlag == "" {
		*adminPasswordFlag = os.Getenv("ADMIN_PASSWORD")
	}
	if *metricsUsernameFlag == "" {
		*metricsUsernameFlag = os.Getenv("METRICS_USERNAME")
	}
	if *metricsPasswordFlag == "" {
		*metricsPasswordFlag = os.Getenv("METRICS_PASSWORD")
	}
	if *metricsAuthFlag == "" {
		*metricsAuthFlag = os.Getenv("METRICS_AUTH")
	}
	if *slackSecretFlag == "" {
		*slackSecretFlag = os.Getenv("SLACK_SECRET")
	}
	if *metricsIdFlag == "" {
		*metricsIdFlag = os.Getenv("HOSTNAME")
	}

	if *metricsProxyURLFlag != "" {
		_, err := url.Parse(*metricsProxyURLFlag)
		if err != nil {
			fmt.Printf("Unable to parse --metrics-proxy-url: %s\n", err.Error())
			os.Exit(2)
		}
	}

	// Contact information
	if *contactFlag == "" {
		fmt.Printf("Contact information, such as an email address, must be specified using the --contact parameter.\n")
		os.Exit(2)
	}

	// mmdb path
	geodb, err := geoip.Init(*mmdbASNPathFlag, *mmdbCityPathFlag)
	if err != nil {
		fmt.Printf("Unable to load geoip database: %s\n", err.Error())
		os.Exit(2)
	}

	// Set database port to 5432 if not set or invalid
	dbport, err := strconv.Atoi(*dbPortFlag)
	if err != nil {
		dbport = 5432
	}

	daoconn, err := dbl.Init(*dbHostFlag, dbport, *dbNameFlag, *dbUsernameFlag, *dbPasswordFlag)
	if err != nil {
		fmt.Printf("Unable to connect to the database: %s\n", err.Error())
		os.Exit(2)
	}

	if err := daoconn.CreateSchema(); err != nil {
		fmt.Printf("Unable to create Schema: %s\n", err.Error())
	}

	s3UrlTtl, err := time.ParseDuration(*s3UrlTtlFlag)
	if err != nil {
		fmt.Printf("Unable to parse --s3-url-ttl: %s\n", err.Error())
		os.Exit(2)
	}
	fmt.Printf("TTL for presigned S3 URLs: %s\n", s3UrlTtl.String())

	filter := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	for _, v := range strings.Fields(*rejectFileExtensions) {
		if !filter.Match([]byte(v)) {
			fmt.Printf("Extension specified by --reject-file-extensions contains illegal characters: %v\n", v)
			os.Exit(2)
		}
		fmt.Printf("Rejecting file extension: %s\n", v)
	}

	s3conn, err := s3.Init(*s3EndpointFlag, *s3BucketFlag, *s3RegionFlag, *s3AccessKeyFlag, *s3SecretKeyFlag, *s3SecureFlag, s3UrlTtl)
	if err != nil {
		fmt.Printf("Unable to initialize S3 connection: %s\n", err.Error())
		os.Exit(2)
	}

	if *s3TraceFlag {
		s3conn.SetTrace(*s3TraceFlag)
	}

	l := &Lurker{
		dao: &daoconn,
		s3:  &s3conn,
	}

	// Start the lurker process
	l.Init(*lurkerIntervalFlag, *logRetentionFlag)
	l.Run()

	u, err := url.Parse(*baseURLFlag)
	if err != nil {
		fmt.Printf("Unable to parse the baseurl parameter: %s\n", *baseURLFlag)
		os.Exit(2)
	}

	config := &ds.Config{
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
		fmt.Printf("Unable to parse the --limit-storage parameter: %s\n", *baseURLFlag)
		os.Exit(2)
	}
	config.LimitStorageReadable = humanize.Bytes(config.LimitStorageBytes)

	// Initialize workspace manager
	wm, err := workspace.NewManager(*tmpdirFlag, *tmpdirThresholdFlag)
	if err != nil {
		fmt.Printf("Unable to initialize workspace manager: %s\n", err.Error())
		os.Exit(2)
	}
	fmt.Printf("Workspace capacity threshold: %.1fx file size\n", *tmpdirThresholdFlag)

	// Create Prometheus registry and metrics
	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics(*metricsIdFlag, metricsRegistry)
	metrics.LimitBytes = config.LimitStorageBytes

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &daoconn,
		s3:              &s3conn,
		geodb:           &geodb,
		workspace:       wm,
		config:          config,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}

	if err := h.Init(); err != nil {
		fmt.Printf("Unable to start the HTTP server: %s\n", err.Error())
	}
	fmt.Printf("Uploaded files expiration: %s\n", config.ExpirationDuration.String())

	// Start the http server
	h.Run()
}
