package s3

import (
	"bytes"
	"strings"
	"testing"
)

const (
	ENDPOINT   = "s3:9000"
	REGION     = "us-east-1"
	BUCKET     = "filebin-test"
	ACCESS_KEY = "s3accesskey"
	SECRET_KEY = "s3secretkey"
)

func tearUp() (S3AO, error) {
	s3ao, err := Init(ENDPOINT, BUCKET, REGION, ACCESS_KEY, SECRET_KEY)
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
	err = tearDown(s3ao)
	if err != nil {
		t.Error(err)
	}
}

func TestFailingInit(t *testing.T) {
	_, err := Init("", "", "", "", "")
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

	name := "testobject"
	content := "content"
	err = s3ao.PutObject(name, strings.NewReader(content), int64(len(content)))
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

	name := "testobject2"
	content := "content2"
	if err := s3ao.PutObject(name, strings.NewReader(content), int64(len(content))); err != nil {
		t.Errorf("Unable to put object: %s\n", err.Error())
	}

	if err := s3ao.RemoveObject(name); err != nil {
		t.Errorf("Unable to remove object: %s\n", err.Error())
	}
}

func TestGetObject(t *testing.T) {
	s3ao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(s3ao)

	name := "testobject"
	content := "content"
	if err := s3ao.PutObject(name, strings.NewReader(content), int64(len(content))); err != nil {
		t.Errorf("Unable to put object: %s\n", err.Error())
	}

	fp, err := s3ao.GetObject(name)
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

	name := "testobject"
	fp, err := s3ao.GetObject(name)
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

	err = s3ao.RemoveObject(name)
	if err != nil {
		// This is strange behaviour. The library should return an error
		// if the object does not exist.
		//t.Errorf("Expected an error when removing a non-existing object")
		t.Errorf("Change in behaviour")
	}
}
