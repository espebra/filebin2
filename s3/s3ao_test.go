package s3

import (
	"bytes"
	"crypto/sha256"
	"fmt"
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
	s3ao, err := Init(ENDPOINT, BUCKET, REGION, ACCESS_KEY, SECRET_KEY, false, expiry)
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
	_, err := Init("", "", "", "", "", false, expiry)
	if err == nil {
		t.Error("Was expecting to fail here, invalid user and db name were provided.")
	}
}

func TestPutObject(t *testing.T) {
	s3ao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(s3ao)

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
	defer tearDown(s3ao)

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
	defer tearDown(s3ao)

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

	buf := new(bytes.Buffer)
	buf.ReadFrom(fp)
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
	defer tearDown(s3ao)

	// Use a non-existent SHA256
	nonExistentSHA256 := "0000000000000000000000000000000000000000000000000000000000000000"
	fp, err := s3ao.GetObject(nonExistentSHA256, 0, 0)
	if err != nil {
		// This is strange behaviour. The library should return an error
		// if the object does not exist.
		//t.Errorf("Expected an error when getting a non-existing object")
		t.Errorf("Change in behaviour")
	}

	// Print contents of the file
	buf := new(bytes.Buffer)
	buf.ReadFrom(fp)
	s := buf.String()
	if len(s) != 0 {
		t.Errorf("Expected empty response, but got %s\n", s)
	}

	err = s3ao.RemoveObjectByHash(nonExistentSHA256)
	if err != nil {
		// This is strange behaviour. The library should return an error
		// if the object does not exist.
		//t.Errorf("Expected an error when removing a non-existing object")
		t.Errorf("Change in behaviour")
	}
}
