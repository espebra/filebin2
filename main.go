package main

import (
	"embed"
	"flag"
	"fmt"
	_ "net/http/pprof"
	"os"
	"strconv"
	//"github.com/espebra/filebin2/ds"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/geoip"
	"github.com/espebra/filebin2/s3"
	"math/rand"
	"net/url"
	"time"
)

var (
	// Various
	expirationFlag      = flag.Int("expiration", 604800, "Bin expiration time in seconds since the last bin update")
	tmpdirFlag          = flag.String("tmpdir", os.TempDir(), "Directory for temporary files for upload and download")
	baseURLFlag         = flag.String("baseurl", "https://filebin.net", "The base URL to use. Required for self-hosted instances.")
	requireApprovalFlag = flag.Bool("manual-approval", false, "Require manual admin approval of new bins before files can be downloaded.")
	mmdbPathFlag        = flag.String("mmdb", "", "The path to an mmdb formatted geoip database like GeoLite2-City.mmdb.")
	allowRobotsFlag     = flag.Bool("allow-robots", false, "Allow robots to crawl and index the site (using X-Robots-Tag response header).")

	// Limits
	limitFileDownloadsFlag = flag.Uint64("limit-file-downloads", 0, "Limit the number of downloads per file. 0 disables this limit.")
	limitStorageFlag       = flag.String("limit-storage", "0", "Limit the storage capacity to use (examples: 100MB, 20GB, 2TB). 0 disables this limit.")

	// HTTP
	listenHostFlag   = flag.String("listen-host", "127.0.0.1", "Listen host")
	listenPortFlag   = flag.Int("listen-port", 8080, "Listen port")
	accessLogFlag    = flag.String("access-log", "/var/log/filebin/access.log", "Path for access.log output")
	proxyHeadersFlag = flag.Bool("proxy-headers", false, "Read client request information from proxy headers")

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
	adminUsernameFlag = flag.String("admin-username", "", "Admin username")
	adminPasswordFlag = flag.String("admin-password", "", "Admin password")

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
	if *slackSecretFlag == "" {
		*slackSecretFlag = os.Getenv("SLACK_SECRET")
	}

	// mmdb path
	geodb, err := geoip.Init(*mmdbPathFlag)
	if err != nil {
		fmt.Printf("Unable to load geoip database: %s\n", err.Error())
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
		AdminPassword:      *adminPasswordFlag,
		AdminUsername:      *adminUsernameFlag,
		AllowRobots:        *allowRobotsFlag,
		BaseUrl:            *u,
		Expiration:         *expirationFlag,
		HttpHost:           *listenHostFlag,
		HttpAccessLog:      *accessLogFlag,
		HttpPort:           *listenPortFlag,
		HttpProxyHeaders:   *proxyHeadersFlag,
		LimitFileDownloads: *limitFileDownloadsFlag,
		RequireApproval:    *requireApprovalFlag,
		SlackSecret:        *slackSecretFlag,
		SlackDomain:        *slackDomainFlag,
		SlackChannel:       *slackChannelFlag,
		Tmpdir:             *tmpdirFlag,
	}

	config.LimitStorageBytes, err = humanize.ParseBytes(*limitStorageFlag)
	if err != nil {
		fmt.Printf("Unable to parse the --limit-storage parameter: %s\n", *baseURLFlag)
		os.Exit(2)
	}
	config.LimitStorageReadable = humanize.Bytes(config.LimitStorageBytes)

	h := &HTTP{
		staticBox:   &staticBox,
		templateBox: &templateBox,
		dao:         &daoconn,
		s3:          &s3conn,
		geodb:       &geodb,
		config:      config,
	}

	if err := h.Init(); err != nil {
		fmt.Printf("Unable to start the HTTP server: %s\n", err.Error())
	}
	fmt.Printf("Uploaded files expiration: %s\n", config.ExpirationDuration.String())

	// Start the http server
	h.Run()
}
