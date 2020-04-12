package main

import (
	"io"
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
	req, _ := http.NewRequest("POST", "http://localhost:8080/", tc.Content)
	req.Header.Set("Filename", tc.Filename)
	req.Header.Set("Bin", tc.Bin)
	req.Header.Set("Content-SHA256", tc.SHA256)
	req.Header.Set("Content-MD5", tc.MD5)
	req.Close = true
	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
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
