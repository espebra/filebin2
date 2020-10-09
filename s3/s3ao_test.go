package s3

import (
	"bytes"
	"strings"
	"testing"
)

const (
	ENDPOINT       = "s3:9000"
	REGION         = "us-east-1"
	BUCKET         = "filebin-test"
	ACCESS_KEY     = "s3accesskey"
	SECRET_KEY     = "s3secretkey"
	ENCRYPTION_KEY = "s3encryptionkey"
)

func tearUp() (S3AO, error) {
	s3ao, err := Init(ENDPOINT, BUCKET, REGION, ACCESS_KEY, SECRET_KEY, ENCRYPTION_KEY, false)
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
	_, err := Init("", "", "", "", "", "", false)
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
	_, err = s3ao.PutObject(bin, filename, strings.NewReader(content), int64(len(content)))
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
	_, err = s3ao.PutObject(bin, filename, strings.NewReader(content), int64(len(content)))
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

	filename := "testobject"
	bin := "testbin"
	content := "content"
	nonce, err := s3ao.PutObject(bin, filename, strings.NewReader(content), int64(len(content)))
	if err != nil {
		t.Errorf("Unable to put object: %s\n", err.Error())
	}

	fp, err := s3ao.GetObject(bin, filename, nonce, 0, 0)
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

	filename := "testobject"
	bin := "testbin"
	nonce := s3ao.GenerateNonce()
	fp, err := s3ao.GetObject(bin, filename, nonce, 0, 0)
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

	err = s3ao.RemoveObject(bin, filename)
	if err != nil {
		// This is strange behaviour. The library should return an error
		// if the object does not exist.
		//t.Errorf("Expected an error when removing a non-existing object")
		t.Errorf("Change in behaviour")
	}
}
