package web

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	//"strings"
	"sync"
	"testing"
	"time"

	"github.com/espebra/filebin2/internal/dbl"
	"github.com/espebra/filebin2/internal/ds"
	"github.com/espebra/filebin2/internal/geoip"
	"github.com/espebra/filebin2/internal/s3"
	"github.com/espebra/filebin2/internal/workspace"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	testLimitFileDownloads = 2
	testLimitStorage       = 10000000
	testExpiredAt          = 5
	testHTTPHost           = "localhost"
	testHTTPPort           = 8080
	testDbName             = "db"
	testDbUser             = "username"
	testDbPassword         = "changeme"
	testDbHost             = "db"
	testDbPort             = 5432
	testS3Endpoint         = "s3:5553"
	testS3Region           = "us-east-1"
	testS3Bucket           = "testbin"
	testS3AccessKey        = "s3accesskey"
	testS3SecretKey        = "s3secretkey"
)

var (
	waitForServer sync.WaitGroup
)

func tearUp() (dao dbl.DAO, s3ao s3.S3AO, err error) {
	dao, err = dbl.Init(testDbHost, testDbPort, testDbName, testDbUser, testDbPassword, 25, 25)
	if err != nil {
		return dao, s3ao, err
	}
	if err := dao.ResetDB(); err != nil {
		return dao, s3ao, err
	}
	s3ao, err = s3.Init(s3.Config{
		Endpoint:             testS3Endpoint,
		Bucket:               testS3Bucket,
		Region:               testS3Region,
		AccessKey:            testS3AccessKey,
		SecretKey:            testS3SecretKey,
		Secure:               false,
		PresignExpiry:        time.Second * 10,
		Timeout:              time.Second * 30,
		TransferTimeout:      time.Minute * 10,
		MultipartPartSize:    64 * 1024 * 1024, // 64 MB
		MultipartConcurrency: 3,
	})
	if err != nil {
		return dao, s3ao, err
	}
	return dao, s3ao, nil
}

func tearDown(dao dbl.DAO) error {
	if err := dao.ResetDB(); err != nil {
		return err
	}
	err := dao.Close()
	return err
}

func startHTTPServer(l net.Listener, wg *sync.WaitGroup, h http.Handler) {
	server := &http.Server{Addr: l.Addr().String(), Handler: h}
	wg.Done()
	_ = server.Serve(l)
}

func TestMain(m *testing.M) {
	dao, s3ao, err := tearUp()
	if err != nil {
		log.Fatal(err)
	}
	c := ds.Config{
		LimitFileDownloads:   testLimitFileDownloads,
		LimitStorageBytes:    testLimitStorage,
		Expiration:           testExpiredAt,
		HttpHost:             testHTTPHost,
		HttpPort:             testHTTPPort,
		RejectFileExtensions: []string{"illegal1", "illegal2"},
		AdminUsername:        "admin",
		AdminPassword:        "changeme",
	}
	// Create Prometheus registry and metrics
	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	geodb, err := geoip.Init("../../mmdb/GeoLite2-ASN.mmdb", "../../mmdb/GeoLite2-City.mmdb")
	if err != nil {
		fmt.Printf("Unable to load geoip database: %s\n", err.Error())
		os.Exit(2)
	}

	// Initialize workspace manager for tests
	wm, err := workspace.NewManager(os.TempDir(), 4.0)
	if err != nil {
		fmt.Printf("Unable to initialize workspace manager: %s\n", err.Error())
		os.Exit(2)
	}

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		geodb:           &geodb,
		workspace:       wm,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		fmt.Printf("Unable to start the HTTP server: %s\n", err.Error())
		os.Exit(2)
	}
	tcpListener, _ := net.Listen("tcp", fmt.Sprintf("%s:%d", h.config.HttpHost, h.config.HttpPort))
	waitForServer.Add(1)
	go startHTTPServer(tcpListener, &waitForServer, h.router)
	retCode := m.Run()
	tcpListener.Close()
	h.Stop()
	_ = tearDown(dao)
	os.Exit(retCode)
}
