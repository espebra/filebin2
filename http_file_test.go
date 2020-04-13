package main

import (
	"log"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"path"
	"net/url"
)

type TestCase struct {
	Method     string
	Filename   string
	Bin        string
	Content    string
	MD5        string
	SHA256     string
	StatusCode int
}

func httpRequest(tc TestCase) (statuscode int, body string, err error) {
	u, err := url.Parse("http://localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	if tc.Method != "POST" {
		if tc.Bin != "" && tc.Filename == "" {
			u.Path = path.Join(u.Path, tc.Bin)
		}
		if tc.Bin != "" && tc.Filename != "" {
			u.Path = path.Join(tc.Bin, tc.Filename)
		}
	}

	var req *http.Request
	if tc.Content == "" {
		req, err = http.NewRequest(tc.Method, u.String(), nil)
	} else {
		req, err = http.NewRequest(tc.Method, u.String(), strings.NewReader(tc.Content))
	}
	if err != nil {
		return -1, "", err
	}
	if tc.Filename != "" {
		req.Header.Set("Filename", tc.Filename)
	}
	if tc.Bin != "" {
		req.Header.Set("Bin", tc.Bin)
	}
	if tc.SHA256 != "" {
		req.Header.Set("Content-SHA256", tc.SHA256)
	}
	if tc.MD5 != "" {
		req.Header.Set("Content-MD5", tc.MD5)
	}
	req.Close = true
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return -2, "", err
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -3, "", err
	}
	body = string(content)
	resp.Body.Close()
	return resp.StatusCode, body, err
}

func TestUploadFile(t *testing.T) {
	tcs := []TestCase{
		{
			// Test case 0: Ok to specify everything
			Method:     "POST",
			Filename:   "a",
			Bin:        "mytestbin",
			Content:    "content a",
			SHA256:     "0069ffe8481777aa403982d9e9b3fa48957015a07cfa0f66dae32050b95bda54",
			MD5:        "d8114b361885ee54897e52ce2308e274",
			StatusCode: 201,
		}, {
			// Test case 1: Ok to not specify MD5 and SHA256
			Method:     "POST",
			Filename:   "b",
			Bin:        "mytestbin",
			Content:    "content b",
			StatusCode: 201,
		}, {
			// Test case 2: Missing filename should fail
			Method:     "POST",
			Filename:   "",
			Bin:        "mytestbin",
			Content:    "some content",
			StatusCode: 400,
		}, {
			// Test case 3: No content should fail
			Method:     "POST",
			Filename:   "c",
			Bin:        "mytestbin",
			Content:    "",
			StatusCode: 400,
		}, {
			// Test case 4: Wrong MD5 checksum should fail
			Method:     "POST",
			Filename:   "d",
			Bin:        "mytestbin",
			Content:    "some more content",
			MD5:        "wrong checksum",
			StatusCode: 400,
		}, {
			// Test case 5: Wrong SHA256 checksum should fail
			Method:     "POST",
			Filename:   "e",
			Bin:        "mytestbin",
			Content:    "some more content",
			SHA256:     "wrong checksum",
			StatusCode: 400,
		}, {
			// Test case 6: New file that will be updated later
			Method:     "POST",
			Filename:   "f",
			Bin:        "mytestbin",
			Content:    "first revision",
			StatusCode: 201,
		}, {
			// Test case 7: New file that will be updated later
			Method:     "POST",
			Filename:   "f",
			Bin:        "mytestbin",
			Content:    "second revision",
			StatusCode: 201,
		},
	}

	for i, tc := range tcs {
		statusCode, _, err := httpRequest(tc)
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
			Method:     "GET",
			Filename:   "a",
			Bin:        "mytestbin",
			Content:    "content a",
			SHA256:     "0069ffe8481777aa403982d9e9b3fa48957015a07cfa0f66dae32050b95bda54",
			MD5:        "d8114b361885ee54897e52ce2308e274",
			StatusCode: 200,
		}, {
			// Test case 1: Unknown file
			Method:     "GET",
			Filename:   "unknown",
			Bin:        "mytestbin",
			StatusCode: 404,
		}, {
			// Test case 2: Unknown bin
			Method:     "GET",
			Filename:   "unknown",
			Bin:        "unknown",
			StatusCode: 404,
		},
	}
	for i, tc := range tcs {
		statusCode, body, err := httpRequest(tc)
		if err != nil {
			t.Errorf("Test case %d: Did not expect http request to fail: %s\n", i, err.Error())
		}
		if tc.StatusCode != statusCode {
			t.Errorf("Test case %d: Expected response code %d, got %d\n", i, tc.StatusCode, statusCode)
		}
		if tc.Content != "" {
			if tc.Content != body {
				t.Errorf("Test case %d: Expected body %s, got %s\n", i, tc.Content, body)
			}
		}
	}
}

func TestDeleteFile(t *testing.T) {
	tc := TestCase{
		Method:     "POST",
		Filename:   "a",
		Bin:        "mytestbin",
		Content:    "content a",
		StatusCode: 201,
	}

	statusCode, _, err := httpRequest(tc)
	if err != nil {
		t.Errorf("Did not expect http request to fail: %s\n", err.Error())
	}
	if tc.StatusCode != statusCode {
		t.Errorf("Expected response code %d, got %d\n", tc.StatusCode, statusCode)
	}

	tc = TestCase{
		Method:     "DELETE",
		Filename:   "a",
		Bin:        "mytestbin",
		StatusCode: 200,
	}
	statusCode, _, err = httpRequest(tc)
	if err != nil {
		t.Errorf("Did not expect http request to fail: %s\n", err.Error())
	}
	if tc.StatusCode != statusCode {
		t.Errorf("Expected response code %d, got %d\n", tc.StatusCode, statusCode)
	}

	tc = TestCase{
		Method:     "GET",
		Filename:   "a",
		Bin:        "mytestbin",
		StatusCode: 404,
	}
	statusCode, _, err = httpRequest(tc)
	if err != nil {
		t.Errorf("Did not expect http request to fail: %s\n", err.Error())
	}
	if tc.StatusCode != statusCode {
		t.Errorf("Expected response code %d, got %d\n", tc.StatusCode, statusCode)
	}
}

func TestLockBin(t *testing.T) {
	tc := TestCase{
		Method:     "POST",
		Filename:   "a",
		Bin:        "mytestbin",
		Content:    "content a",
		StatusCode: 201,
	}
	statusCode, _, err := httpRequest(tc)
	if err != nil {
		t.Errorf("Did not expect http request to fail: %s\n", err.Error())
	}
	if tc.StatusCode != statusCode {
		t.Errorf("Expected response code %d, got %d\n", tc.StatusCode, statusCode)
	}

	tc = TestCase{
		Method:     "LOCK",
		Bin:        "mytestbin",
		StatusCode: 200,
	}
	statusCode, _, err = httpRequest(tc)
	if err != nil {
		t.Errorf("Did not expect http request to fail: %s\n", err.Error())
	}
	if tc.StatusCode != statusCode {
		t.Errorf("Expected response code %d, got %d\n", tc.StatusCode, statusCode)
	}

	tc = TestCase{
		Method:     "POST",
		Filename:   "b",
		Bin:        "mytestbin",
		Content:    "content a",
		StatusCode: 405,
	}
	statusCode, _, err = httpRequest(tc)
	if err != nil {
		t.Errorf("Did not expect http request to fail: %s\n", err.Error())
	}
	if tc.StatusCode != statusCode {
		t.Errorf("Expected response code %d, got %d\n", tc.StatusCode, statusCode)
	}
}
