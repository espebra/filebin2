package main

import (
	"os"
	"fmt"
	"log"
	"net"
	"sync"
	"strings"
	"testing"
	"net/http"
	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/s3"
	"github.com/GeertJohan/go.rice"
)

const (
	testHTTPHost        = "localhost"
	testHTTPPort        = 8080
	testDbName          = "db"
	testDbUser          = "username"
	testDbPassword      = "changeme"
	testDbHost          = "db"
	testDbPort          = 5432
	testS3Endpoint      = "s3:9000"
	testS3Region        = "us-east-1"
	testS3Bucket        = "testbin"
	testS3AccessKey     = "s3accesskey"
	testS3SecretKey     = "s3secretkey"
	testS3EncryptionKey = "encryptionkey"
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
	s3ao, err = s3.Init(testS3Endpoint, testS3Bucket, testS3Region, testS3AccessKey, testS3SecretKey, testS3EncryptionKey)
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

func startHttpServer(l net.Listener, wg *sync.WaitGroup, h http.Handler) {
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
	h := &HTTP{
		httpHost:    testHTTPHost,
		httpPort:    testHTTPPort,
		staticBox:   staticBox,
		templateBox: templateBox,
		dao:         &dao,
		s3:          &s3ao,
	}
	if err := h.Init(); err != nil {
		fmt.Printf("Unable to start the HTTP server: %s\n", err.Error())
		os.Exit(2)
	}
	tcpListener, _ := net.Listen("tcp", fmt.Sprintf("%s:%d", h.httpHost, h.httpPort))
	waitForServer.Add(1)
	go startHttpServer(tcpListener, &waitForServer, h.router)
	retCode := m.Run()
	tcpListener.Close()
	tearDown(dao)
	os.Exit(retCode)
}

//func uploadFile(bin, filename, content, sha256, md5 string, size int) error {
//}

func TestUploadFile(t *testing.T) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	filename := "a"
	bin := "mytestbin"
	content := "content a"
	req, _ := http.NewRequest("POST", "http://localhost:8080/", strings.NewReader(content))
	req.Header.Set("Filename", filename)
	req.Header.Set("Bin", bin)
	req.Header.Set("Content-SHA256", "8bfe5f10912d733d91a002a4f9990bd72ff03120817d46742f810c0484d626ef")
	req.Header.Set("Content-MD5", "d19f7ae40de729f92bf9aea2657d1c77")
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("Did not expect file upload to fail: %s\n", err.Error())
	}
	if resp.StatusCode != 201 {
		t.Errorf("Expected response code 201, got %d\n", resp.StatusCode)
	}
	resp.Body.Close()
	req.Close = true
}
