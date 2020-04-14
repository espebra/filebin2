package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"
)

type TestCase struct {
	Method     string
	Bin        string
	Filename   string
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
			Bin:        "mytestbin",
			Filename:   "a",
			Content:    "content a",
			SHA256:     "0069ffe8481777aa403982d9e9b3fa48957015a07cfa0f66dae32050b95bda54",
			MD5:        "d8114b361885ee54897e52ce2308e274",
			StatusCode: 201,
		}, {
			// Test case 1: Ok to not specify MD5 and SHA256
			Method:     "POST",
			Bin:        "mytestbin",
			Filename:   "b",
			Content:    "content b",
			StatusCode: 201,
		}, {
			// Test case 2: Missing filename should fail
			Method:     "POST",
			Bin:        "mytestbin",
			Filename:   "",
			Content:    "some content",
			StatusCode: 400,
		}, {
			// Test case 3: No content should fail
			Method:     "POST",
			Bin:        "mytestbin",
			Filename:   "c",
			Content:    "",
			StatusCode: 400,
		}, {
			// Test case 4: Wrong MD5 checksum should fail
			Method:     "POST",
			Bin:        "mytestbin",
			Filename:   "d",
			Content:    "some more content",
			MD5:        "wrong checksum",
			StatusCode: 400,
		}, {
			// Test case 5: Wrong SHA256 checksum should fail
			Method:     "POST",
			Bin:        "mytestbin",
			Filename:   "e",
			Content:    "some more content",
			SHA256:     "wrong checksum",
			StatusCode: 400,
		}, {
			// Test case 6: New file that will be updated later
			Method:     "POST",
			Bin:        "mytestbin",
			Filename:   "f",
			Content:    "first revision",
			StatusCode: 201,
		}, {
			// Test case 7: New file that will be updated later
			Method:     "POST",
			Bin:        "mytestbin",
			Filename:   "f",
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
			Bin:        "mytestbin",
			Filename:   "a",
			Content:    "content a",
			SHA256:     "0069ffe8481777aa403982d9e9b3fa48957015a07cfa0f66dae32050b95bda54",
			MD5:        "d8114b361885ee54897e52ce2308e274",
			StatusCode: 200,
		}, {
			// Test case 1: Unknown file
			Method:     "GET",
			Bin:        "mytestbin",
			Filename:   "unknown",
			StatusCode: 404,
		}, {
			// Test case 2: Unknown bin
			Method:     "GET",
			Bin:        "unknown",
			Filename:   "unknown",
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
		Bin:        "mytestbin",
		Filename:   "a",
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
		Bin:        "mytestbin",
		Filename:   "a",
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
		Bin:        "mytestbin",
		Filename:   "a",
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

func TestLockAndDeleteBin(t *testing.T) {
	tc := TestCase{
		Method:     "POST",
		Bin:        "mytestbin",
		Filename:   "a",
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

	tcs := []TestCase{
		{
			// Try to update existing file, should be rejected
			Method:     "POST",
			Bin:        "mytestbin",
			Filename:   "a",
			Content:    "content a",
			StatusCode: 405,
		}, {
			// Try to create new file, should be rejected
			Method:     "POST",
			Bin:        "mytestbin",
			Filename:   "b",
			Content:    "content b",
			StatusCode: 405,
		},
	}
	for _, tc := range tcs {
		statusCode, _, err = httpRequest(tc)
		if err != nil {
			t.Errorf("Did not expect http request to fail: %s\n", err.Error())
		}
		if tc.StatusCode != statusCode {
			t.Errorf("Expected response code %d, got %d\n", tc.StatusCode, statusCode)
		}
	}

	// Delete bin
	tc = TestCase{
		Method:     "DELETE",
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

	// Fetch the deleted file
	tc = TestCase{
		Method:     "GET",
		Bin:        "mytestbin",
		Filename:   "a",
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
