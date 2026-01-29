package s3

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

const (
	ENDPOINT   = "s3:5553"
	REGION     = "us-east-1"
	BUCKET     = "filebin-test"
	ACCESS_KEY = "s3accesskey"
	SECRET_KEY = "s3secretkey"
)

func tearUp() (S3AO, error) {
	expiry := time.Second * 10
	timeout := time.Second * 30
	transferTimeout := time.Minute * 10
	partSize := int64(64 * 1024 * 1024) // 64 MB
	concurrency := 3
	s3ao, err := Init(ENDPOINT, BUCKET, REGION, ACCESS_KEY, SECRET_KEY, false, expiry, timeout, transferTimeout, partSize, concurrency)
	if err != nil {
		return s3ao, err
	}
	return s3ao, nil
}

func tearDown(s3ao S3AO) error {
	err := s3ao.RemoveBucket()
	if err != nil {
		return err
	}
	return nil
}

func TestInit(t *testing.T) {
	s3ao, err := tearUp()
	if err != nil {
		t.Error(err)
	}

	s3ao.SetTrace(true)

	status := s3ao.Status()
	if status == false {
		t.Error("Was expecting status to be true here")
	}

	err = tearDown(s3ao)
	if err != nil {
		t.Error(err)
	}
}

func TestFailingInit(t *testing.T) {
	expiry := time.Second * 10
	timeout := time.Second * 30
	transferTimeout := time.Minute * 10
	partSize := int64(64 * 1024 * 1024)
	concurrency := 3
	_, err := Init("", "", "", "", "", false, expiry, timeout, transferTimeout, partSize, concurrency)
	if err == nil {
		t.Error("Was expecting to fail here, invalid user and db name were provided.")
	}
}

func TestPutObject(t *testing.T) {
	s3ao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(s3ao) }()

	filename := "testobject"
	bin := "testbin"
	content := "content"
	err = s3ao.PutObject(bin, filename, strings.NewReader(content), int64(len(content)))
	if err != nil {
		t.Errorf("Unable to put file: %s\n", err.Error())
	}
}

func TestRemoveObject(t *testing.T) {
	s3ao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(s3ao) }()

	filename := "testobject2"
	bin := "testbin2"
	content := "content2"
	err = s3ao.PutObject(bin, filename, strings.NewReader(content), int64(len(content)))
	if err != nil {
		t.Errorf("Unable to put object: %s\n", err.Error())
	}

	if err := s3ao.RemoveObject(bin, filename); err != nil {
		t.Errorf("Unable to remove object: %s\n", err.Error())
	}
}

func TestGetObject(t *testing.T) {
	s3ao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(s3ao) }()

	content := "content"
	// Calculate SHA256 of content
	h := sha256.New()
	h.Write([]byte(content))
	contentSHA256 := fmt.Sprintf("%x", h.Sum(nil))

	if err := s3ao.PutObjectByHash(contentSHA256, strings.NewReader(content), int64(len(content))); err != nil {
		t.Errorf("Unable to put object: %s\n", err.Error())
	}

	fp, err := s3ao.GetObject(contentSHA256, 0, 0)
	if err != nil {
		t.Errorf("Unable to get object: %s\n", err.Error())
	}
	defer fp.Close()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(fp)
	s := buf.String()
	if content != s {
		t.Errorf("Invalid content from get object. Expected %s, got %s\n", content, s)
	}
}

func TestUnknownObject(t *testing.T) {
	s3ao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(s3ao) }()

	// Use a non-existent SHA256
	nonExistentSHA256 := "0000000000000000000000000000000000000000000000000000000000000000"
	fp, err := s3ao.GetObject(nonExistentSHA256, 0, 0)
	if err == nil {
		// AWS SDK returns an error when the object does not exist (unlike Minio SDK)
		// Close the reader if we got one
		if fp != nil {
			fp.Close()
		}
		t.Errorf("Expected an error when getting a non-existing object")
	}

	// Verify the error is a "not found" type error
	// AWS SDK v2 returns NoSuchKey error for non-existent objects
	if fp != nil {
		// If we somehow got a reader, verify it's empty
		buf := new(bytes.Buffer)
		_, _ = io.Copy(buf, fp)
		fp.Close()
		s := buf.String()
		if len(s) != 0 {
			t.Errorf("Expected empty response, but got %s\n", s)
		}
	}

	// RemoveObjectByHash on non-existent object should not error in S3
	err = s3ao.RemoveObjectByHash(nonExistentSHA256)
	if err != nil {
		// S3 DeleteObject doesn't return an error for non-existent objects
		// This is expected AWS S3 behavior
		t.Errorf("Unexpected error when removing non-existing object: %s", err)
	}
}

func TestMultipartUpload(t *testing.T) {
	// Initialize with a 5 MB part size to force multipart upload for our test data
	expiry := time.Second * 10
	timeout := time.Second * 30
	transferTimeout := time.Minute * 10
	partSize := int64(5 * 1024 * 1024) // 5 MB
	concurrency := 3
	s3ao, err := Init(ENDPOINT, BUCKET+"-multipart", REGION, ACCESS_KEY, SECRET_KEY, false, expiry, timeout, transferTimeout, partSize, concurrency)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s3ao.RemoveBucket() }()

	// Create ~11 MB of content to exceed the 5 MB part size threshold
	contentSize := 11 * 1024 * 1024
	content := make([]byte, contentSize)
	for i := range content {
		content[i] = byte(i % 256)
	}

	// Calculate SHA256 of content
	h := sha256.New()
	h.Write(content)
	contentSHA256 := fmt.Sprintf("%x", h.Sum(nil))

	// Upload using PutObjectByHash (this should trigger multipart upload)
	err = s3ao.PutObjectByHash(contentSHA256, bytes.NewReader(content), int64(contentSize))
	if err != nil {
		t.Fatalf("Unable to put object via multipart upload: %s", err)
	}

	// Download and verify content matches
	fp, err := s3ao.GetObject(contentSHA256, 0, 0)
	if err != nil {
		t.Fatalf("Unable to get object: %s", err)
	}
	defer fp.Close()

	downloaded, err := io.ReadAll(fp)
	if err != nil {
		t.Fatalf("Unable to read downloaded object: %s", err)
	}

	if len(downloaded) != contentSize {
		t.Errorf("Downloaded size mismatch: expected %d, got %d", contentSize, len(downloaded))
	}

	if !bytes.Equal(content, downloaded) {
		t.Error("Downloaded content does not match uploaded content")
	}
}
