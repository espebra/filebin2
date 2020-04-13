package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

type TestCase struct {
	Filename   string
	Bin        string
	Content    io.Reader
	MD5        string
	SHA256     string
	StatusCode int
}

func upload(tc TestCase) (int, error) {
	req, err := http.NewRequest("POST", "http://localhost:8080/", tc.Content)
	if err != nil {
		return -1, err
	}
	req.Header.Set("Filename", tc.Filename)
	req.Header.Set("Bin", tc.Bin)
	req.Header.Set("Content-SHA256", tc.SHA256)
	req.Header.Set("Content-MD5", tc.MD5)
	req.Close = true
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return resp.StatusCode, err
	}
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	return resp.StatusCode, err
}

func download(tc TestCase) (int, error) {
	url := fmt.Sprintf("http://localhost:8080/%s/%s", tc.Bin, tc.Filename)
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return -1, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return -2, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	buf := new(bytes.Buffer)
	buf.ReadFrom(tc.Content)
	if string(body) != buf.String() {
		return resp.StatusCode, errors.New(fmt.Sprintf("Content does not match. Got %s, expected %s", string(body), buf.String()))
	}
	return resp.StatusCode, err
}

func TestUploadFile(t *testing.T) {
	tcs := []TestCase{
		{
			// Test case 0: Ok to specify everything
			Filename:   "a",
			Bin:        "mytestbin",
			Content:    strings.NewReader("content a"),
			SHA256:     "0069ffe8481777aa403982d9e9b3fa48957015a07cfa0f66dae32050b95bda54",
			MD5:        "d8114b361885ee54897e52ce2308e274",
			StatusCode: 201,
		}, {
			// Test case 1: Ok to not specify MD5 and SHA256
			Filename:   "b",
			Bin:        "mytestbin",
			Content:    strings.NewReader("content b"),
			StatusCode: 201,
		}, {
			// Test case 2: Missing filename should fail
			Filename:   "",
			Bin:        "mytestbin",
			Content:    strings.NewReader("some content"),
			StatusCode: 400,
		}, {
			// Test case 3: No content should fail
			Filename:   "c",
			Bin:        "mytestbin",
			Content:    strings.NewReader(""),
			StatusCode: 400,
		}, {
			// Test case 4: Wrong MD5 checksum should fail
			Filename:   "d",
			Bin:        "mytestbin",
			Content:    strings.NewReader("some more content"),
			MD5:        "wrong checksum",
			StatusCode: 400,
		}, {
			// Test case 5: Wrong SHA256 checksum should fail
			Filename:   "e",
			Bin:        "mytestbin",
			Content:    strings.NewReader("some more content"),
			SHA256:     "wrong checksum",
			StatusCode: 400,
		}, {
			// Test case 6: New file that will be updated later
			Filename:   "f",
			Bin:        "mytestbin",
			Content:    strings.NewReader("first revision"),
			StatusCode: 201,
		}, {
			// Test case 7: New file that will be updated later
			Filename:   "f",
			Bin:        "mytestbin",
			Content:    strings.NewReader("second revision"),
			StatusCode: 201,
		},
	}

	for i, tc := range tcs {
		statusCode, err := upload(tc)
		if err != nil {
			t.Errorf("Test case %d: Did not expect http request to fail: %s\n", i, err.Error())
		}
		if statusCode != tc.StatusCode {
			t.Errorf("Test case %d: Expected response code %d, got %d\n", i, tc.StatusCode, statusCode)
		}
	}
}

func TestDownloadFile(t *testing.T) {
	tcs := []TestCase{
		{
			// Test case 0: Ok to specify everything
			Filename:   "a",
			Bin:        "mytestbin",
			Content:    strings.NewReader("content a"),
			SHA256:     "0069ffe8481777aa403982d9e9b3fa48957015a07cfa0f66dae32050b95bda54",
			MD5:        "d8114b361885ee54897e52ce2308e274",
			StatusCode: 200,
		},
	}

	for i, tc := range tcs {
		statusCode, err := download(tc)
		if err != nil {
			t.Errorf("Test case %d: Did not expect http request to fail: %s\n", i, err.Error())
		}
		if statusCode != tc.StatusCode {
			t.Errorf("Test case %d: Expected response code %d, got %d\n", i, tc.StatusCode, statusCode)
		}
	}
}
