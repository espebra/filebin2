package main

import (
	"fmt"
	"github.com/GeertJohan/go.rice"
	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/s3"
	"log"
	"net"
	"net/http"
	"os"
	//"strings"
	"sync"
	"testing"
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
	testS3Endpoint         = "s3:9000"
	testS3Region           = "us-east-1"
	testS3Bucket           = "testbin"
	testS3AccessKey        = "s3accesskey"
	testS3SecretKey        = "s3secretkey"
	testS3EncryptionKey    = "encryptionkey"
)

var (
	waitForServer sync.WaitGroup
)

func tearUp() (dao dbl.DAO, s3ao s3.S3AO, err error) {
	dao, err = dbl.Init(testDbHost, testDbPort, testDbName, testDbUser, testDbPassword)
	if err != nil {
		return dao, s3ao, err
	}
	if err := dao.ResetDB(); err != nil {
		return dao, s3ao, err
	}
	s3ao, err = s3.Init(testS3Endpoint, testS3Bucket, testS3Region, testS3AccessKey, testS3SecretKey, testS3EncryptionKey, false)
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
	server.Serve(l)
}

func TestMain(m *testing.M) {
	dao, s3ao, err := tearUp()
	if err != nil {
		log.Fatal(err)
	}
	staticBox := rice.MustFindBox("static")
	templateBox := rice.MustFindBox("templates")
	c := ds.Config{
		LimitFileDownloads: testLimitFileDownloads,
		LimitStorageBytes:  testLimitStorage,
		Expiration:         testExpiredAt,
		HttpHost:           testHTTPHost,
		HttpPort:           testHTTPPort,
	}
	h := &HTTP{
		staticBox:   staticBox,
		templateBox: templateBox,
		dao:         &dao,
		s3:          &s3ao,
		config:      &c,
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
	tearDown(dao)
	os.Exit(retCode)
}
