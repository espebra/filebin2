package web

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
)

type TestCase struct {
	Description     string
	Method          string
	Bin             string
	Filename        string
	UploadContent   string
	DownloadContent string
	MD5             string
	SHA256          string
	StatusCode      int
}

func (tc TestCase) String() string {
	return fmt.Sprintf("Test case details:\n\nDescription: %s\nMethod: %s\nBin: %s\nFilename: %s\nUpload content: %s\nDownload content: %s\nMD5: %s\nSHA256: %s\nExpected status code: %d\n\n", tc.Description, tc.Method, tc.Bin, tc.Filename, tc.UploadContent, tc.DownloadContent, tc.MD5, tc.SHA256, tc.StatusCode)
}

func httpRequest(tc TestCase) (statuscode int, body string, err error) {
	u, err := url.Parse("http://localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	if tc.Bin != "" {
		u.Path = path.Join(u.Path, tc.Bin)
	}
	if tc.Filename != "" {
		u.Path = path.Join(u.Path, tc.Filename)
	}

	var req *http.Request
	if tc.UploadContent == "" {
		req, err = http.NewRequest(tc.Method, u.String(), nil)
	} else {
		req, err = http.NewRequest(tc.Method, u.String(), strings.NewReader(tc.UploadContent))
		if tc.SHA256 != "" {
			fmt.Printf("Content-SHA256 request header: %s\n", tc.SHA256)
			req.Header.Set("Content-SHA256", tc.SHA256)
		}
		if tc.MD5 != "" {
			md5Bytes, err := hex.DecodeString(tc.MD5)
			var checksum string
			if err != nil {
				// Not valid hex — send as-is to trigger a mismatch
				checksum = tc.MD5
			} else {
				checksum = base64.StdEncoding.EncodeToString(md5Bytes)
			}
			fmt.Printf("Content-MD5 request header: %s\n", checksum)
			req.Header.Set("Content-MD5", checksum)
		}
	}
	if err != nil {
		return -1, "", err
	}

	for name, values := range req.Header {
		for _, value := range values {
			fmt.Printf("Request header: %s=%s\n", name, value)
		}
	}

	req.Close = true
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return -2, "", err
	}

	//dump, err := httputil.DumpResponse(resp, true)
	//if err != nil {
	//	log.Fatal(err)
	//}

	//fmt.Printf("Response headers: %q", dump)
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return -3, "", err
	}
	body = string(content)
	resp.Body.Close()
	return resp.StatusCode, body, err
}

func runTests(tcs []TestCase, t *testing.T) {
	for i, tc := range tcs {
		statusCode, body, err := httpRequest(tc)
		if err != nil {
			t.Errorf("Test case %d: Did not expect http request to fail: %s\n", i, err.Error())
			t.Errorf("%s\n", tc.String())
			os.Exit(1)
		}
		if tc.StatusCode != statusCode {
			t.Errorf("Test case %d\n", i)
			t.Errorf("  Expected response code %d, got %d\n", tc.StatusCode, statusCode)
			t.Errorf("  Response body: %s\n", body)
			t.Errorf("  %s\n", tc.String())
			os.Exit(1)
		}
		if tc.DownloadContent != "" {
			if tc.DownloadContent != body {
				t.Errorf("Test case %d: Expected body %s, got %s\n", i, tc.DownloadContent, body)
				t.Errorf("%s\n", tc.String())
				os.Exit(1)
			}
		}
	}
}

func TestUploadFile(t *testing.T) {
	tcs := []TestCase{
		{
			Description:   "Upload ok to specify everything",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "a",
			UploadContent: "content a",
			SHA256:        "0069ffe8481777aa403982d9e9b3fa48957015a07cfa0f66dae32050b95bda54",
			MD5:           "d8114b361885ee54897e52ce2308e274",
			StatusCode:    201,
		}, {
			Description:   "Ok to not specify MD5 and SHA256",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "b",
			UploadContent: "content b",
			StatusCode:    201,
		}, {
			Description:   "Missing filename should fail",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "",
			UploadContent: "some content",
			StatusCode:    405,
		}, {
			Description:   "No content should fail",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "c",
			UploadContent: "",
			StatusCode:    400,
		}, {
			Description:   "Wrong MD5 checksum should fail",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "d",
			UploadContent: "some more content",
			MD5:           "wrong checksum",
			StatusCode:    400,
		}, {
			Description:   "Wrong SHA256 checksum should fail",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "e",
			UploadContent: "some more content",
			SHA256:        "wrong checksum",
			StatusCode:    400,
		}, {
			Description:   "Upload new file that will be updated later",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "f",
			UploadContent: "first revision",
			StatusCode:    201,
		}, {
			Description:   "Update file that will be updated later",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "f",
			UploadContent: "second revision",
			StatusCode:    201,
		}, {
			Description:     "Ok to specify everything on download",
			Method:          "GET",
			Bin:             "mytestbin",
			Filename:        "a",
			DownloadContent: "content a",
			StatusCode:      200,
		}, {
			Description: "Try to download non-existing file",
			Method:      "GET",
			Bin:         "mytestbin",
			Filename:    "unknown",
			StatusCode:  404,
		}, {
			Description: "Try to view non-existing bin",
			Method:      "GET",
			Bin:         "unknown",
			Filename:    "unknown",
			StatusCode:  404,
		},
	}
	runTests(tcs, t)
}

func TestUploadToDeletedAtBin(t *testing.T) {
	tcs := []TestCase{
		{
			Description:   "Create file to set up test case",
			Method:        "POST",
			Bin:           "mytestbin2",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    201,
		}, {
			Description: "Delete bin",
			Method:      "DELETE",
			Bin:         "mytestbin2",
			StatusCode:  200,
		}, {
			Description:   "Create the file again, it should fail",
			Method:        "POST",
			Bin:           "mytestbin2",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    405,
		},
	}
	runTests(tcs, t)
}

func TestDeleteFile(t *testing.T) {
	tcs := []TestCase{
		{
			Description:   "Create file to set up test case",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    201,
		}, {
			Description: "Delete file before trying to fetch it",
			Method:      "DELETE",
			Bin:         "mytestbin",
			Filename:    "a",
			StatusCode:  200,
		}, {
			Description: "Get file after it was deleted, should fail",
			Method:      "GET",
			Bin:         "mytestbin",
			Filename:    "a",
			StatusCode:  404,
		},
	}
	runTests(tcs, t)
}

func TestLockAndDeleteBin(t *testing.T) {
	tcs := []TestCase{
		{
			Description: "Fetch bin that does not exist",
			Method:      "GET",
			Bin:         "mytestbin",
			StatusCode:  200,
		}, {
			Description:   "Create file to set up test case",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    201,
		}, {
			Description: "Lock the bin",
			Method:      "PUT",
			Bin:         "mytestbin",
			StatusCode:  200,
		}, {
			Description: "Lock the bin again",
			Method:      "PUT",
			Bin:         "mytestbin",
			StatusCode:  200,
		}, {
			Description:   "Try to update existing file, should be rejected",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    405,
		}, {
			Description:   "Try to create a new file, should be rejected",
			Method:        "POST",
			Bin:           "mytestbin",
			Filename:      "b",
			UploadContent: "content b",
			StatusCode:    405,
		}, {
			Description: "Delete bin, should be accepted",
			Method:      "DELETE",
			Bin:         "mytestbin",
			StatusCode:  200,
		}, {
			Description: "Delete bin again, should not be found",
			Method:      "DELETE",
			Bin:         "mytestbin",
			StatusCode:  404,
		}, {
			Description: "Get the file from the bin that was deleted, should fail",
			Method:      "GET",
			Bin:         "mytestbin",
			Filename:    "a",
			StatusCode:  404,
		}, {
			Description: "Delete the bin that was deleted, should be not found",
			Method:      "DELETE",
			Bin:         "mytestbin",
			StatusCode:  404,
		}, {
			Description: "Lock the bin that was deleted, should fail",
			Method:      "PUT",
			Bin:         "mytestbin",
			StatusCode:  404,
		},
	}
	runTests(tcs, t)
}

func TestNotExistingBinsAndFiles(t *testing.T) {
	tcs := []TestCase{
		{
			Description: "Get bin that doesn't exist",
			Method:      "GET",
			Bin:         "unknownbin",
			StatusCode:  200,
		}, {
			Description: "Lock bin that doesn't exist",
			Method:      "PUT",
			Bin:         "unknownbin",
			StatusCode:  404,
		}, {
			Description: "Delete bin that doesn't exist",
			Method:      "DELETE",
			Bin:         "unknownbin",
			StatusCode:  404,
		}, {
			Description: "Delete file that doesn't exist in bin that doesn't exist",
			Method:      "DELETE",
			Bin:         "unknownbin2",
			Filename:    "unknownfile",
			StatusCode:  404,
		}, {
			Description:   "Create new bin",
			Method:        "POST",
			Bin:           "mytestbin3",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    201,
		}, {
			Description: "Get bin",
			Method:      "GET",
			Bin:         "mytestbin3",
			StatusCode:  200,
		}, {
			Description:     "Get file",
			Method:          "GET",
			Bin:             "mytestbin3",
			Filename:        "a",
			DownloadContent: "content a",
			StatusCode:      200,
		}, {
			Description: "Delete file that doesn't exist in bin that exists",
			Method:      "DELETE",
			Bin:         "mytestbin3",
			Filename:    "unknownfile",
			StatusCode:  404,
		}, {
			Description: "Delete file that exists in bin that exists",
			Method:      "DELETE",
			Bin:         "mytestbin3",
			Filename:    "a",
			StatusCode:  200,
		}, {
			Description: "Delete file again that no longer exists in bin that exists",
			Method:      "DELETE",
			Bin:         "mytestbin3",
			Filename:    "a",
			StatusCode:  404,
		}, {
			Description: "Get file that was deleted",
			Method:      "GET",
			Bin:         "mytestbin3",
			Filename:    "a",
			StatusCode:  404,
		}, {
			Description:   "Create file again",
			Method:        "POST",
			Bin:           "mytestbin3",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    201,
		}, {
			Description: "Delete bin",
			Method:      "DELETE",
			Bin:         "mytestbin3",
			StatusCode:  200,
		}, {
			Description: "Get the bin that was deleted",
			Method:      "GET",
			Bin:         "mytestbin3",
			StatusCode:  404,
		}, {
			Description: "Get file from bin that was deleted",
			Method:      "GET",
			Bin:         "mytestbin3",
			Filename:    "a",
			StatusCode:  404,
		}, {
			Description: "Delete file from the bin that is deleted",
			Method:      "DELETE",
			Bin:         "mytestbin3",
			Filename:    "a",
			StatusCode:  404,
		}, {
			Description: "Lock bin that is deleted",
			Method:      "PUT",
			Bin:         "mytestbin3",
			StatusCode:  404,
		},
	}
	runTests(tcs, t)
}

func TestLimitFileDownloads(t *testing.T) {
	tcs := []TestCase{
		{
			Description:   "Create new bin",
			Method:        "POST",
			Bin:           "mytestbin4",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    201,
		}, {
			Description:     "Get file first time",
			Method:          "GET",
			Bin:             "mytestbin4",
			Filename:        "a",
			DownloadContent: "content a",
			StatusCode:      200,
		}, {
			Description:     "Get file second time",
			Method:          "GET",
			Bin:             "mytestbin4",
			Filename:        "a",
			DownloadContent: "content a",
			StatusCode:      200,
		}, {
			Description: "Get file third time, above the download limit of 2",
			Method:      "GET",
			Bin:         "mytestbin4",
			Filename:    "a",
			StatusCode:  403,
		},
	}
	runTests(tcs, t)
}

func TestBinInputValidation(t *testing.T) {
	tcs := []TestCase{
		{
			Description:   "Too long bin",
			Method:        "POST",
			Bin:           "yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    400,
		},
		{
			Description:   "Too short bin",
			Method:        "POST",
			Bin:           "yyyy",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    400,
		},
		{
			Description:   "Bin with invalid characters",
			Method:        "POST",
			Bin:           "asdf$%&=^*",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    404,
		},
		{
			Description:   "Bin with reserved name (/admin/)",
			Method:        "POST",
			Bin:           "admin",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    400,
		},
		{
			Description:   "Bin with reserved name (/archive/)",
			Method:        "POST",
			Bin:           "archive",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    400,
		},
		{
			Description:   "File with illegal extension)",
			Method:        "POST",
			Bin:           "1234567890",
			Filename:      "a.illegal1",
			UploadContent: "content a",
			StatusCode:    403,
		},
		{
			Description:   "File with illegal extension)",
			Method:        "POST",
			Bin:           "1234567890",
			Filename:      "b.illegal2",
			UploadContent: "content b",
			StatusCode:    403,
		},
	}
	runTests(tcs, t)
}

func TestBinText(t *testing.T) {
	tcs := []TestCase{
		{
			Description:   "Create new bin",
			Method:        "POST",
			Bin:           "mytestbin6",
			Filename:      "a",
			UploadContent: "content a",
			StatusCode:    201,
		}, {
			Description:   "Upload second file",
			Method:        "POST",
			Bin:           "mytestbin6",
			Filename:      "b",
			UploadContent: "content b",
			StatusCode:    201,
		}, {
			Description:     "Get bin in plaintext",
			Method:          "GET",
			Bin:             "mytestbin6.txt",
			DownloadContent: "/mytestbin6/a\n/mytestbin6/b\n",
			StatusCode:      200,
		},
	}
	runTests(tcs, t)
}

func httpAdminRequest(method, path string) (statuscode int, body string, err error) {
	u, err := url.Parse("http://localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	u.Path = path

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return -1, "", err
	}

	// Add basic auth for admin endpoints
	req.SetBasicAuth("admin", "changeme")
	req.Close = true

	// Don't follow redirects - return the redirect response directly
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return -2, "", err
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return -3, "", err
	}
	body = string(content)
	resp.Body.Close()
	return resp.StatusCode, body, err
}

func TestBlockContent(t *testing.T) {
	// First, upload a file with known content
	tcs := []TestCase{
		{
			Description:   "Upload file with specific content",
			Method:        "POST",
			Bin:           "blockedcontenttest",
			Filename:      "blocked.txt",
			UploadContent: "this content will be blocked",
			SHA256:        "11758cbca80f40e79c605f12c9094c3b2ed9329f5e3ed41d920b11f941de995d",
			StatusCode:    201,
		},
	}
	runTests(tcs, t)

	// Block the content using the admin endpoint
	sha256 := "11758cbca80f40e79c605f12c9094c3b2ed9329f5e3ed41d920b11f941de995d"
	statusCode, body, err := httpAdminRequest("POST", "/admin/file/"+sha256+"/block")
	if err != nil {
		t.Errorf("Failed to block content: %s\n", err.Error())
		os.Exit(1)
	}
	if statusCode != 303 && statusCode != 200 {
		t.Errorf("Expected status code 303 or 200 when blocking content, got %d\n", statusCode)
		t.Errorf("Response body: %s\n", body)
		os.Exit(1)
	}

	// Try to upload the same content again - should be rejected
	tcs2 := []TestCase{
		{
			Description:   "Try to upload blocked content to same bin",
			Method:        "POST",
			Bin:           "blockedcontenttest",
			Filename:      "blocked2.txt",
			UploadContent: "this content will be blocked",
			SHA256:        "11758cbca80f40e79c605f12c9094c3b2ed9329f5e3ed41d920b11f941de995d",
			StatusCode:    403,
		},
		{
			Description:   "Try to upload blocked content to different bin",
			Method:        "POST",
			Bin:           "anotherbin",
			Filename:      "test.txt",
			UploadContent: "this content will be blocked",
			SHA256:        "11758cbca80f40e79c605f12c9094c3b2ed9329f5e3ed41d920b11f941de995d",
			StatusCode:    403,
		},
		{
			Description:   "Upload different content should still work",
			Method:        "POST",
			Bin:           "blockedcontenttest",
			Filename:      "allowed.txt",
			UploadContent: "this content is allowed",
			StatusCode:    201,
		},
	}
	runTests(tcs2, t)
}

func TestUnblockContent(t *testing.T) {
	// First, upload a file with known content
	tcs := []TestCase{
		{
			Description:   "Upload file with specific content",
			Method:        "POST",
			Bin:           "unblockedcontenttest",
			Filename:      "unblocked.txt",
			UploadContent: "this content will be unblocked",
			SHA256:        "54755318a367ec28f632443fa35dad11d3bdc626aa6cc7afa0b91fc4aaa8f034",
			StatusCode:    201,
		},
	}
	runTests(tcs, t)

	// Block the content using the admin endpoint
	sha256 := "54755318a367ec28f632443fa35dad11d3bdc626aa6cc7afa0b91fc4aaa8f034"
	statusCode, body, err := httpAdminRequest("POST", "/admin/file/"+sha256+"/block")
	if err != nil {
		t.Errorf("Failed to block content: %s\n", err.Error())
		os.Exit(1)
	}
	if statusCode != 303 && statusCode != 200 {
		t.Errorf("Expected status code 303 or 200 when blocking content, got %d\n", statusCode)
		t.Errorf("Response body: %s\n", body)
		os.Exit(1)
	}

	// Verify upload is rejected while blocked
	tcs2 := []TestCase{
		{
			Description:   "Try to upload blocked content",
			Method:        "POST",
			Bin:           "unblockedcontenttest2",
			Filename:      "blocked.txt",
			UploadContent: "this content will be unblocked",
			SHA256:        "54755318a367ec28f632443fa35dad11d3bdc626aa6cc7afa0b91fc4aaa8f034",
			StatusCode:    403,
		},
	}
	runTests(tcs2, t)

	// Unblock the content
	statusCode, body, err = httpAdminRequest("POST", "/admin/file/"+sha256+"/unblock")
	if err != nil {
		t.Errorf("Failed to unblock content: %s\n", err.Error())
		os.Exit(1)
	}
	if statusCode != 303 && statusCode != 200 {
		t.Errorf("Expected status code 303 or 200 when unblocking content, got %d\n", statusCode)
		t.Errorf("Response body: %s\n", body)
		os.Exit(1)
	}

	// Verify upload is now allowed after unblocking
	tcs3 := []TestCase{
		{
			Description:   "Upload previously blocked content after unblock",
			Method:        "POST",
			Bin:           "unblockedcontenttest3",
			Filename:      "nowallowed.txt",
			UploadContent: "this content will be unblocked",
			SHA256:        "54755318a367ec28f632443fa35dad11d3bdc626aa6cc7afa0b91fc4aaa8f034",
			StatusCode:    201,
		},
	}
	runTests(tcs3, t)
}

func TestBinBan(t *testing.T) {
	tcs := []TestCase{
		{
			Description:   "Create new bin",
			Method:        "POST",
			Bin:           "mytestbin5",
			Filename:      "aaa",
			UploadContent: "content a",
			StatusCode:    201,
		}, {
			Description:     "Get file first time",
			Method:          "GET",
			Bin:             "mytestbin5",
			Filename:        "aaa",
			DownloadContent: "content a",
			StatusCode:      200,
		}, {
			Description: "Ban bin",
			Method:      "BAN",
			Bin:         "mytestbin5",
			StatusCode:  200,
		}, {
			Description: "Get bin after being banned",
			Method:      "GET",
			Bin:         "mytestbin5",
			StatusCode:  404,
		}, {
			Description: "Get file after being banned",
			Method:      "GET",
			Bin:         "mytestbin5",
			Filename:    "aaa",
			StatusCode:  403,
		}, {
			Description:   "Create new bin after being banned",
			Method:        "POST",
			Bin:           "mytestbin6",
			Filename:      "aaa",
			UploadContent: "content a",
			StatusCode:    403,
		},
	}
	runTests(tcs, t)
}
