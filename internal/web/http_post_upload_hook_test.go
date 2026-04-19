package web

import (
	"crypto/sha256"
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

func setupPostUploadHookHandler(t *testing.T, postUploadHook string, postUploadHookTimeout time.Duration) *HTTP {
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
		Expiration:            testExpiredAt,
		PostUploadHook:        postUploadHook,
		PostUploadHookTimeout: postUploadHookTimeout,
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
	f, err := os.CreateTemp("", "post-upload-hook-*.sh")
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

func TestPostUploadHookAccept(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
exit 0
`)
	h := setupPostUploadHookHandler(t, hook, 10*time.Second)

	req := uploadRequest("/hookacceptbin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestPostUploadHookFailureDoesNotBlockUpload(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
echo "hook reports something"
exit 1
`)
	h := setupPostUploadHookHandler(t, hook, 10*time.Second)

	req := uploadRequest("/hookfailurebin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestPostUploadHookNotConfigured(t *testing.T) {
	h := setupPostUploadHookHandler(t, "", 10*time.Second)

	req := uploadRequest("/hooknonebin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestPostUploadHookTimeoutDoesNotBlockUpload(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
sleep 10
exit 0
`)
	h := setupPostUploadHookHandler(t, hook, 1*time.Second)

	req := uploadRequest("/hooktimeoutbin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestPostUploadHookReceivesArguments(t *testing.T) {
	// Hook script writes its arguments to a temp file so we can verify them
	argsFile, err := os.CreateTemp("", "hook-args-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	argsFilePath := argsFile.Name()
	_ = argsFile.Close()
	t.Cleanup(func() { _ = os.Remove(argsFilePath) })

	hook := writeHookScript(t, `#!/bin/sh
printf '%s\n' "$@" > `+argsFilePath+`
exit 0
`)
	h := setupPostUploadHookHandler(t, hook, 10*time.Second)

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

	got := map[string]string{}
	for i := 0; i+1 < len(lines); i += 2 {
		got[lines[i]] = lines[i+1]
	}

	expectedSHA256 := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	expected := map[string]string{
		"--bin-id":   "hookargsbin",
		"--filename": "myfile.txt",
		"--size":     "17",
		"--sha256":   expectedSHA256,
	}
	for flag, want := range expected {
		if got[flag] != want {
			t.Errorf("Expected %s=%q, got %q", flag, want, got[flag])
		}
	}
	if !strings.Contains(got["--content-type"], "text/plain") {
		t.Errorf("Expected --content-type containing %q, got %q", "text/plain", got["--content-type"])
	}
	if _, ok := got["--bin-id"]; !ok {
		t.Errorf("Expected --bin-id flag to be present: %q", string(argsContent))
	}
}

// TestPostUploadHookRunsAfterPersistence verifies that when the hook executes,
// the file has already been persisted to the database.
func TestPostUploadHookRunsAfterPersistence(t *testing.T) {
	marker, err := os.CreateTemp("", "hook-marker-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	markerPath := marker.Name()
	_ = marker.Close()
	t.Cleanup(func() { _ = os.Remove(markerPath) })

	hook := writeHookScript(t, `#!/bin/sh
echo "ran" > `+markerPath+`
exit 0
`)
	h := setupPostUploadHookHandler(t, hook, 10*time.Second)

	req := uploadRequest("/hookpersistbin/persist.txt", "persisted content")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	file, found, err := h.dao.File().GetByName("hookpersistbin", "persist.txt")
	if err != nil {
		t.Fatalf("Failed to load file: %v", err)
	}
	if !found {
		t.Fatal("Expected file to be persisted before hook returned")
	}
	if !file.InStorage {
		t.Error("Expected file to be marked as stored before hook returned")
	}

	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("Failed to read hook marker: %v", err)
	}
	if strings.TrimSpace(string(data)) != "ran" {
		t.Errorf("Hook marker not written: %q", string(data))
	}
}
