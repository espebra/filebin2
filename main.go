package main

import (
	"flag"
	"fmt"
	"os"
	//"github.com/espebra/filebin2/ds"
	"github.com/GeertJohan/go.rice"
	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/s3"
	"math/rand"
	"time"
)

var (
	expirationFlag = flag.Int("expiration", 604800, "Bin expiration time in seconds since the last bin update")

	// Limits
	limitFileDownloadsFlag = flag.Uint64("limit-file-downloads", 0, "Limit the number of downloads per file. 0 disables this limit.")
	limitStorageFlag       = flag.Uint64("limit-storage", 0, "Limit the storage capacity to use, in number of gigabytes. 0 disables this limit.")

	// HTTP
	listenHostFlag   = flag.String("listen-host", "127.0.0.1", "Listen host")
	listenPortFlag   = flag.Int("listen-port", 8080, "Listen port")
	accessLogFlag    = flag.String("access-log", "", "Path for access.log output")
	proxyHeadersFlag = flag.Bool("proxy-headers", false, "Read client request information from proxy headers")

	// Database
	dbHostFlag     = flag.String("db-host", "127.0.0.1", "Database host")
	dbPortFlag     = flag.Int("db-port", 5432, "Database port")
	dbNameFlag     = flag.String("db-name", os.Getenv("DATABASE_NAME"), "Name of the database")
	dbUsernameFlag = flag.String("db-username", os.Getenv("DATABASE_USERNAME"), "Database username")
	dbPasswordFlag = flag.String("db-password", os.Getenv("DATABASE_PASSWORD"), "Database password")

	// S3
	s3EndpointFlag      = flag.String("s3-endpoint", os.Getenv("S3_ENDPOINT"), "S3 endpoint")
	s3BucketFlag        = flag.String("s3-bucket", os.Getenv("S3_BUCKET"), "S3 bucket")
	s3RegionFlag        = flag.String("s3-region", os.Getenv("S3_REGION"), "S3 region")
	s3AccessKeyFlag     = flag.String("s3-access-key", os.Getenv("S3_ACCESS_KEY"), "S3 access key")
	s3SecretKeyFlag     = flag.String("s3-secret-key", os.Getenv("S3_SECRET_KEY"), "S3 secret key")
	s3EncryptionKeyFlag = flag.String("s3-encryption-key", os.Getenv("S3_ENCRYPTION_KEY"), "S3 encryption key")
	s3TraceFlag         = flag.Bool("s3-trace", false, "Enable S3 HTTP tracing for debugging")

	// Lurker
	lurkerIntervalFlag = flag.Int("lurker-interval", 300, "Lurker interval is the delay to sleep between each run in seconds")

	// Auth
	adminUsernameFlag = flag.String("admin-username", os.Getenv("ADMIN_USERNAME"), "Admin username")
	adminPasswordFlag = flag.String("admin-password", os.Getenv("ADMIN_PASSWORD"), "Admin password")
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	flag.Parse()

	daoconn, err := dbl.Init(*dbHostFlag, *dbPortFlag, *dbNameFlag, *dbUsernameFlag, *dbPasswordFlag)
	if err != nil {
		fmt.Printf("Unable to connect to the database: %s\n", err.Error())
		os.Exit(2)
	}

	if err := daoconn.CreateSchema(); err != nil {
		fmt.Printf("Unable to create Schema: %s\n", err.Error())
	}

	s3conn, err := s3.Init(*s3EndpointFlag, *s3BucketFlag, *s3RegionFlag, *s3AccessKeyFlag, *s3SecretKeyFlag, *s3EncryptionKeyFlag)
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
	l.Init(*lurkerIntervalFlag)
	l.Run()

	staticBox := rice.MustFindBox("static")
	templateBox := rice.MustFindBox("templates")

	h := &HTTP{
		httpHost:           *listenHostFlag,
		httpPort:           *listenPortFlag,
		httpAccessLog:      *accessLogFlag,
		httpProxyHeaders:   *proxyHeadersFlag,
		adminUsername:      *adminUsernameFlag,
		adminPassword:      *adminPasswordFlag,
		staticBox:          staticBox,
		templateBox:        templateBox,
		dao:                &daoconn,
		s3:                 &s3conn,
		expiration:         *expirationFlag,
		limitFileDownloads: *limitFileDownloadsFlag,
		limitStorage:       *limitStorageFlag,
	}

	if err := h.Init(); err != nil {
		fmt.Printf("Unable to start the HTTP server: %s\n", err.Error())
	}
	fmt.Printf("Expiration: %s\n", h.expirationDuration.String())

	// Start the http server
	h.Run()
}
