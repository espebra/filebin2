package web

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/espebra/filebin2/internal/ds"
)

func setupPreUploadHookHandler(t *testing.T, preUploadHook string, preUploadHookTimeout time.Duration) *HTTP {
	t.Helper()
	return setupUploadHookHandler(t, ds.Config{
		PreUploadHook:        preUploadHook,
		PreUploadHookTimeout: preUploadHookTimeout,
	})
}

// assertNotPersisted fails the test if the file or its content was stored.
func assertNotPersisted(t *testing.T, h *HTTP, bin, filename, content string) {
	t.Helper()
	if _, found, err := h.dao.File().GetByName(bin, filename); err != nil {
		t.Fatalf("Failed to look up file: %v", err)
	} else if found {
		t.Errorf("Expected file %s/%s to not be persisted after rejection", bin, filename)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	if _, found, err := h.dao.FileContent().GetBySHA256(sum); err != nil {
		t.Fatalf("Failed to look up file content: %v", err)
	} else if found {
		t.Errorf("Expected content %s to not be persisted after rejection", sum)
	}
	if _, err := h.s3.StatObject(sum); err == nil {
		t.Errorf("Expected content %s to not be in S3 after rejection", sum)
	}
}

func TestPreUploadHookAccept(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
exit 0
`)
	h := setupPreUploadHookHandler(t, hook, 10*time.Second)

	req := uploadRequest("/prehookacceptbin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
	assertPersisted(t, h, "prehookacceptbin", "testfile.txt")
}

func TestPreUploadHookNotConfigured(t *testing.T) {
	h := setupPreUploadHookHandler(t, "", 10*time.Second)

	req := uploadRequest("/prehooknonebin/testfile.txt", "hello world")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestPreUploadHookRejectReturnsForbiddenWithMessage(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
echo "some diagnostic output"
echo "This file type is not welcome here"
echo "only for the log" >&2
exit 1
`)
	h := setupPreUploadHookHandler(t, hook, 10*time.Second)

	content := "rejected content"
	req := uploadRequest("/prehookrejectbin/testfile.txt", content)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}
	if got := strings.TrimSpace(rr.Body.String()); got != "This file type is not welcome here" {
		t.Errorf("Expected the last line of stdout as the message, got %q", got)
	}
	assertNotPersisted(t, h, "prehookrejectbin", "testfile.txt", content)
}

func TestPreUploadHookRejectWithoutOutputUsesDefaultMessage(t *testing.T) {
	hook := writeHookScript(t, `#!/bin/sh
exit 1
`)
	h := setupPreUploadHookHandler(t, hook, 10*time.Second)

	req := uploadRequest("/prehooksilentbin/testfile.txt", "silently rejected")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}
	if got := strings.TrimSpace(rr.Body.String()); got != "The upload was rejected" {
		t.Errorf("Expected the default rejection message, got %q", got)
	}
}

// assertPersisted fails the test if the file was not stored.
func assertPersisted(t *testing.T, h *HTTP, bin, filename string) {
	t.Helper()
	if _, found, err := h.dao.File().GetByName(bin, filename); err != nil {
		t.Fatalf("Failed to look up file: %v", err)
	} else if !found {
		t.Errorf("Expected file %s/%s to be persisted", bin, filename)
	}
}

func TestPreUploadHookOtherExitCodeAcceptsUpload(t *testing.T) {
	for _, code := range []int{2, 3, 127, 255} {
		t.Run(fmt.Sprintf("exit_%d", code), func(t *testing.T) {
			hook := writeHookScript(t, fmt.Sprintf(`#!/bin/sh
echo "something broke"
exit %d
`, code))
			h := setupPreUploadHookHandler(t, hook, 10*time.Second)

			bin := fmt.Sprintf("prehookexit%dbin", code)
			req := uploadRequest("/"+bin+"/testfile.txt", "content behind a broken hook")
			rr := httptest.NewRecorder()
			h.router.ServeHTTP(rr, req)

			if rr.Code != http.StatusCreated {
				t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
			}
			assertPersisted(t, h, bin, "testfile.txt")
		})
	}
}

func TestPreUploadHookMissingCommandAcceptsUpload(t *testing.T) {
	h := setupPreUploadHookHandler(t, "/nonexistent/pre-upload-hook", 10*time.Second)

	req := uploadRequest("/prehookmissingbin/testfile.txt", "content behind a missing hook")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
	assertPersisted(t, h, "prehookmissingbin", "testfile.txt")
}

func TestPreUploadHookTimeoutAcceptsUpload(t *testing.T) {
	// The hook would reject the upload if it got to finish, so a 201 shows
	// that the timeout accepted it rather than the hook.
	hook := writeHookScript(t, `#!/bin/sh
sleep 10
echo "too late to reject"
exit 1
`)
	h := setupPreUploadHookHandler(t, hook, 1*time.Second)

	req := uploadRequest("/prehooktimeoutbin/testfile.txt", "content behind a slow hook")
	rr := httptest.NewRecorder()
	start := time.Now()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Errorf("Expected the hook to be killed at the timeout, request took %s", elapsed)
	}
	assertPersisted(t, h, "prehooktimeoutbin", "testfile.txt")
}

func TestPreUploadHookReceivesArguments(t *testing.T) {
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
	h := setupPreUploadHookHandler(t, hook, 10*time.Second)

	content := "test content here"
	req := uploadRequest("/prehookargsbin/myfile.txt", content)
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

	expected := map[string]string{
		"--bin-id":   "prehookargsbin",
		"--filename": "myfile.txt",
		"--size":     "17",
		"--md5":      fmt.Sprintf("%x", md5.Sum([]byte(content))),
		"--sha1":     fmt.Sprintf("%x", sha1.Sum([]byte(content))),
		"--sha256":   fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
	}
	for flag, want := range expected {
		if got[flag] != want {
			t.Errorf("Expected %s=%q, got %q", flag, want, got[flag])
		}
	}
	if !strings.Contains(got["--content-type"], "text/plain") {
		t.Errorf("Expected --content-type containing %q, got %q", "text/plain", got["--content-type"])
	}
}

// TestPreUploadHookRejectSkipsPostUploadHook verifies that the post-upload
// hook is not notified about an upload the pre-upload hook rejected.
func TestPreUploadHookRejectSkipsPostUploadHook(t *testing.T) {
	marker, err := os.CreateTemp("", "post-hook-marker-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	markerPath := marker.Name()
	_ = marker.Close()
	t.Cleanup(func() { _ = os.Remove(markerPath) })

	preHook := writeHookScript(t, `#!/bin/sh
exit 1
`)
	postHook := writeHookScript(t, `#!/bin/sh
echo "post hook ran" > `+markerPath+`
exit 0
`)
	h := setupUploadHookHandler(t, ds.Config{
		PreUploadHook:         preHook,
		PreUploadHookTimeout:  10 * time.Second,
		PostUploadHook:        postHook,
		PostUploadHookTimeout: 10 * time.Second,
	})

	req := uploadRequest("/prehookskippostbin/testfile.txt", "rejected before post hook")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}
	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("Failed to read post hook marker: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("Expected the post-upload hook to not run after a rejection, marker: %q", string(data))
	}
}
