package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPError(t *testing.T) {
	cause := errors.New("connection refused")
	err := fmt.Errorf("select bin: %w", &httpError{status: http.StatusServiceUnavailable, message: "Try again later", err: cause})

	var he *httpError
	if !errors.As(err, &he) {
		t.Fatal("expected to find an httpError in the chain")
	}
	if he.status != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want %d", he.status, http.StatusServiceUnavailable)
	}
	if he.message != "Try again later" {
		t.Errorf("message: got %q", he.message)
	}
	if !errors.Is(err, cause) {
		t.Error("expected the cause to be reachable through the chain")
	}
	if got, want := err.Error(), "select bin: Try again later: connection refused"; got != want {
		t.Errorf("Error(): got %q, want %q", got, want)
	}
}

// TestUploadErrorFormats verifies that a rejected upload is reported in the
// format the client asked for through the Accept header.
func TestUploadErrorFormats(t *testing.T) {
	tcs := []struct {
		accept      string
		contentType string
		check       func(t *testing.T, body string)
	}{
		{
			accept:      "",
			contentType: "text/plain; charset=utf-8",
			check: func(t *testing.T, body string) {
				if body != "MD5 checksum did not match" {
					t.Errorf("unexpected plain text body %q", body)
				}
			},
		},
		{
			accept:      "text/html",
			contentType: "text/html; charset=utf-8",
			check: func(t *testing.T, body string) {
				if !strings.Contains(body, "MD5 checksum did not match") {
					t.Errorf("html body does not contain the message: %q", body)
				}
			},
		},
	}

	for _, tc := range tcs {
		t.Run("accept="+tc.accept, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, testServerURL+"/errorformats/a", strings.NewReader("content"))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-MD5", "d29udCBtYXRjaA==")
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status: got %d, want %d, body %q", resp.StatusCode, http.StatusBadRequest, body)
			}
			if got := resp.Header.Get("Content-Type"); got != tc.contentType {
				t.Errorf("content type: got %q, want %q", got, tc.contentType)
			}
			tc.check(t, string(body))
		})
	}
}

// TestUploadToLockedBinSetsAllow verifies that the Allow header is sent along
// with the 405 for uploads to locked bins.
func TestUploadToLockedBinSetsAllow(t *testing.T) {
	bin := "allowheaderbin"
	if code, body, err := httpRequest(TestCase{Method: http.MethodPost, Bin: bin, Filename: "a", UploadContent: "content"}); err != nil || code != http.StatusCreated {
		t.Fatalf("upload failed: %d %s %v", code, body, err)
	}
	if code, body, err := httpRequest(TestCase{Method: http.MethodPut, Bin: bin}); err != nil || code != http.StatusOK {
		t.Fatalf("lock failed: %d %s %v", code, body, err)
	}

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/"+bin+"/b", strings.NewReader("content"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
	if got := resp.Header.Get("Allow"); got != "GET, HEAD" {
		t.Errorf("Allow header: got %q, want %q", got, "GET, HEAD")
	}
}
