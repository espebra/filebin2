package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/espebra/filebin2/internal/ds"
	"github.com/espebra/filebin2/internal/geoip"
	"github.com/espebra/filebin2/internal/workspace"
	"github.com/prometheus/client_golang/prometheus"
)

func setupUploadHookHandler(t *testing.T, uploadHook string, uploadHookTimeout time.Duration) *HTTP {
	t.Helper()

	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tearDown(dao) })

	geodb, err := geoip.Init("../../mmdb/GeoLite2-ASN.mmdb", "../../mmdb/GeoLite2-City.mmdb")
	if err != nil {
		t.Fatalf("Unable to load geoip database: %s", err)
	}

	wm, err := workspace.NewManager(os.TempDir(), 4.0)
	if err != nil {
		t.Fatalf("Unable to initialize workspace manager: %s", err)
	}

	c := ds.Config{
		Expiration:        testExpiredAt,
		UploadHook:        uploadHook,
		UploadHookTimeout: uploadHookTimeout,
	}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		geodb:           &geodb,
		workspace:       wm,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}
	t.Cleanup(func() { h.Stop() })

	return h
}

func writeHookScript(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "upload-hook-*.sh")
	if err != nil {
		t.Fatalf("Failed to create hook script: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("Failed to write hook script: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Failed to close hook script: %v", err)
	}
	if err := os.Chmod(f.Name(), 0755); err != nil {
		t.Fatalf("Failed to chmod hook script: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	return f.Name()
}

func uploadRequest(path, content string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(content))
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(content)))
	return req
}

func TestUploadHookAccept(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
exit 0
`)
	h := setupUploadHookHandler(t, hook, 10*time.Second)

	req := uploadRequest("/hookacceptbin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestUploadHookReject(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
echo "File rejected by policy"
exit 1
`)
	h := setupUploadHookHandler(t, hook, 10*time.Second)

	req := uploadRequest("/hookrejectbin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	if body != "File rejected by policy" {
		t.Errorf("Expected body %q, got %q", "File rejected by policy", body)
	}
}

func TestUploadHookRejectDefaultMessage(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
exit 1
`)
	h := setupUploadHookHandler(t, hook, 10*time.Second)

	req := uploadRequest("/hookrejectdefaultbin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	if body != "Upload rejected" {
		t.Errorf("Expected body %q, got %q", "Upload rejected", body)
	}
}

func TestUploadHookNotConfigured(t *testing.T) {
	h := setupUploadHookHandler(t, "", 10*time.Second)

	req := uploadRequest("/hooknonebin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestUploadHookTimeout(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
sleep 10
exit 0
`)
	h := setupUploadHookHandler(t, hook, 1*time.Second)

	req := uploadRequest("/hooktimeoutbin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
	}
}

func TestUploadHookInternalError(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
echo "Something went wrong"
exit 2
`)
	h := setupUploadHookHandler(t, hook, 10*time.Second)

	req := uploadRequest("/hookerrorbin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	if body != "Something went wrong" {
		t.Errorf("Expected body %q, got %q", "Something went wrong", body)
	}
}

func TestUploadHookRejectUsesLastLine(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
echo "first line"
echo "middle line"
echo "last line is the reason"
exit 1
`)
	h := setupUploadHookHandler(t, hook, 10*time.Second)

	req := uploadRequest("/hooklastlinebin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	if body != "last line is the reason" {
		t.Errorf("Expected body %q, got %q", "last line is the reason", body)
	}
}

func TestUploadHookReceivesArguments(t *testing.T) {
	// Hook script writes its arguments to a temp file so we can verify them
	argsFile, err := os.CreateTemp("", "hook-args-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	argsFilePath := argsFile.Name()
	_ = argsFile.Close()
	t.Cleanup(func() { _ = os.Remove(argsFilePath) })

	hook := writeHookScript(t, `#!/bin/sh
printf '%s\n' "$1" "$2" "$3" "$4" "$5" > `+argsFilePath+`
exit 0
`)
	h := setupUploadHookHandler(t, hook, 10*time.Second)

	content := "test content here"
	req := uploadRequest("/hookargsbin/myfile.txt", content)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	argsContent, err := os.ReadFile(argsFilePath)
	if err != nil {
		t.Fatalf("Failed to read args file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(argsContent)), "\n")

	if len(lines) != 5 {
		t.Fatalf("Expected 5 arguments, got %d: %q", len(lines), string(argsContent))
	}
	if lines[0] != "hookargsbin" {
		t.Errorf("Expected bin ID %q, got %q", "hookargsbin", lines[0])
	}
	if lines[1] != "myfile.txt" {
		t.Errorf("Expected filename %q, got %q", "myfile.txt", lines[1])
	}
	if !strings.Contains(lines[2], "text/plain") {
		t.Errorf("Expected content type containing %q, got %q", "text/plain", lines[2])
	}
	if lines[3] != "17" {
		t.Errorf("Expected file size %q, got %q", "17", lines[3])
	}
	if lines[4] == "" {
		t.Error("Expected non-empty temp file path")
	}
}
