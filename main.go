package main

import (
	"flag"
	"fmt"
	"os"
	//"github.com/espebra/filebin2/ds"
	"github.com/GeertJohan/go.rice"
	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/s3"
)

var (
	// HTTP
	listenHostFlag = flag.String("listen-host", "127.0.0.1", "Listen host")
	listenPortFlag = flag.Int("listen-port", 8080, "Listen port")

	// Database
	dbHostFlag     = flag.String("db-host", "127.0.0.1", "Database host")
	dbPortFlag     = flag.Int("db-port", 5432, "Database port")
	dbNameFlag     = flag.String("db-name", os.Getenv("DATABASE_NAME"), "Name of the database")
	dbUsernameFlag = flag.String("db-username", os.Getenv("DATABASE_USERNAME"), "Database username")
	dbPasswordFlag = flag.String("db-password", os.Getenv("DATABASE_PASSWORD"), "Database password")

	// S3
	s3EndpointFlag  = flag.String("s3-endpoint", os.Getenv("S3_ENDPOINT"), "S3 endpoint")
	s3BucketFlag    = flag.String("s3-bucket", os.Getenv("S3_BUCKET"), "S3 bucket")
	s3RegionFlag    = flag.String("s3-region", os.Getenv("S3_REGION"), "S3 region")
	s3AccessKeyFlag = flag.String("s3-access-key", os.Getenv("S3_ACCESS_KEY"), "S3 access key")
	s3SecretKeyFlag = flag.String("s3-secret-key", os.Getenv("S3_SECRET_KEY"), "S3 secret key")
)

func main() {
	flag.Parse()

	daoconn, err := dbl.Init(*dbHostFlag, *dbPortFlag, *dbNameFlag, *dbUsernameFlag, *dbPasswordFlag)
	if err != nil {
		fmt.Errorf("Unable to connect to the database: %s\n", err.Error())
	}

	if err := daoconn.CreateSchema(); err != nil {
		fmt.Errorf("Unable to create Schema: %s\n", err.Error())
	}

	s3conn, err := s3.Init(*s3EndpointFlag, *s3BucketFlag, *s3RegionFlag, *s3AccessKeyFlag, *s3SecretKeyFlag)
	if err != nil {
		fmt.Errorf("Unable to connect to S3: %s\n", err.Error())
	}

	staticBox := rice.MustFindBox("static")
	templateBox := rice.MustFindBox("templates")

	h := &HTTP{
		httpHost:    *listenHostFlag,
		httpPort:    *listenPortFlag,
		staticBox:   staticBox,
		templateBox: templateBox,
		dao:         &daoconn,
		s3:          &s3conn,
	}

	if err := h.Init(); err != nil {
		fmt.Errorf("Unable to start the HTTP server: %s\n", err.Error())
	}

	h.Run()
}
