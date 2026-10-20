package web

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/espebra/filebin2/internal/audit"
	"github.com/espebra/filebin2/internal/phash"
	"github.com/espebra/filebin2/internal/s3"
)

func readAuditEvents(t *testing.T, dir string) []audit.Event {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "audit-*.jsonl"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("Expected one audit log file, got %v (%v)", paths, err)
	}
	f, err := os.Open(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var events []audit.Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e audit.Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("Invalid audit log line: %s: %q", err, scanner.Text())
		}
		events = append(events, e)
	}
	return events
}

func TestAuditLog(t *testing.T) {
	h := setupActionsHandler(t, nil)
	dir := t.TempDir()
	auditLog, err := audit.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	h.SetAuditLogger(auditLog)

	bin := "auditlogbin"
	content := "content for the audit log"
	uploadReq := uploadRequest("/"+bin+"/file.txt", content)
	uploadReq.Header.Set("Content-Type", "text/plain")
	uploadRR := httptest.NewRecorder()
	h.router.ServeHTTP(uploadRR, uploadReq)
	if uploadRR.Code != http.StatusCreated {
		t.Fatalf("Expected upload status %d, got %d. Body: %s", http.StatusCreated, uploadRR.Code, uploadRR.Body.String())
	}

	serve := func(method, path, userAgent string, admin bool, headers ...string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = "192.0.2.10:54321"
		req.Header.Set("User-Agent", userAgent)
		for i := 0; i+1 < len(headers); i += 2 {
			req.Header.Add(headers[i], headers[i+1])
		}
		if admin {
			req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
		}
		rr := httptest.NewRecorder()
		h.router.ServeHTTP(rr, req)
		return rr
	}

	download := serve(http.MethodGet, "/"+bin+"/file.txt", "audit-agent/1.0", false,
		"X-Forwarded-For", "203.0.113.5",
		"X-Forwarded-For", "198.51.100.7",
		"Forwarded", "for=203.0.113.5",
		"Via", "1.1 proxy.example",
		"Accept-Language", "nb-NO,nb;q=0.9,en;q=0.8",
		"Sec-CH-UA", `"Chromium";v="130"`,
		"Sec-CH-UA-Platform", `"Linux"`,
		"Sec-CH-UA-Mobile", "?0",
		"Origin", "https://example.com",
		"Sec-Fetch-Site", "cross-site",
	)
	if download.Code != http.StatusFound {
		t.Fatalf("Expected download to give %d, got %d", http.StatusFound, download.Code)
	}
	if rr := serve(http.MethodGet, "/archive/"+bin+"/zip", "audit-agent/1.0", false); rr.Code != http.StatusOK {
		t.Fatalf("Expected archive download to give %d, got %d", http.StatusOK, rr.Code)
	}
	// Requests for files that do not exist are not logged
	if rr := serve(http.MethodGet, "/"+bin+"/missing.txt", "audit-agent/1.0", false); rr.Code != http.StatusNotFound {
		t.Fatalf("Expected missing file to give %d, got %d", http.StatusNotFound, rr.Code)
	}
	serve(http.MethodPut, "/admin/approve/"+bin, "admin-agent/1.0", true)

	auditLog.Close()
	events := readAuditEvents(t, dir)

	byName := map[string]audit.Event{}
	for _, e := range events {
		if e.Bin != bin {
			t.Errorf("Unexpected bin in event: %+v", e)
		}
		if e.RequestID == "" {
			t.Errorf("Expected a request id in event: %+v", e)
		}
		if e.Event != "archive_file_included" && e.Outcome != audit.OutcomeComplete {
			t.Errorf("Expected outcome %q in event: %+v", audit.OutcomeComplete, e)
		}
		if _, ok := byName[e.Event]; ok {
			t.Errorf("Unexpected duplicate event: %+v", e)
		}
		byName[e.Event] = e
	}
	if len(events) != 5 {
		t.Fatalf("Expected 5 events, got %d: %+v", len(events), events)
	}

	md5Sum := md5.Sum([]byte(content))
	sha256Sum := sha256.Sum256([]byte(content))

	upload, ok := byName["file_upload"]
	if !ok {
		t.Fatalf("Expected a file_upload event, got %+v", events)
	}
	if upload.Method != http.MethodPost || upload.Status != http.StatusCreated || upload.Filename != "file.txt" || upload.Actor != audit.ActorClient {
		t.Errorf("Unexpected upload event: %+v", upload)
	}
	if upload.MD5 != hex.EncodeToString(md5Sum[:]) || upload.SHA256 != hex.EncodeToString(sha256Sum[:]) || upload.SHA1 == "" {
		t.Errorf("Unexpected checksums in upload event: %+v", upload)
	}
	if upload.DurationSeconds <= 0 || !upload.Completed.After(upload.Time) {
		t.Errorf("Expected a positive duration in seconds for the upload, got %f", upload.DurationSeconds)
	}
	if upload.ReqBytes != int64(len(content)) {
		t.Errorf("Expected %d request bytes for the upload, got %d", len(content), upload.ReqBytes)
	}

	dl, ok := byName["file_download_requested"]
	if !ok {
		t.Fatalf("Expected a file_download_requested event, got %+v", events)
	}
	if dl.Status != http.StatusFound || dl.SHA256 != hex.EncodeToString(sha256Sum[:]) {
		t.Errorf("Unexpected download event: %+v", dl)
	}
	if dl.IP != "192.0.2.10" || dl.UserAgent != "audit-agent/1.0" {
		t.Errorf("Unexpected client details in download event: %+v", dl)
	}
	if dl.XForwardedFor != "203.0.113.5, 198.51.100.7" || dl.Forwarded != "for=203.0.113.5" || dl.Via != "1.1 proxy.example" {
		t.Errorf("Unexpected proxy headers in download event: %+v", dl)
	}
	if dl.AcceptLanguage != "nb-NO,nb;q=0.9,en;q=0.8" || dl.SecCHUA != `"Chromium";v="130"` || dl.SecCHUAPlatform != `"Linux"` || dl.SecCHUAMobile != "?0" {
		t.Errorf("Unexpected client headers in download event: %+v", dl)
	}
	if dl.Origin != "https://example.com" || dl.SecFetchSite != "cross-site" {
		t.Errorf("Unexpected origin headers in download event: %+v", dl)
	}
	if upload.ContentType != "text/plain" {
		t.Errorf("Expected the content type of the upload to be logged: %+v", upload)
	}
	if dl.RequestID != download.Header().Get("X-Request-Id") {
		t.Errorf("Expected request id %q in download event, got %q", download.Header().Get("X-Request-Id"), dl.RequestID)
	}
	// The request id is embedded in the presigned URL, to match the
	// download with S3 access logs
	location, err := url.Parse(download.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Unable to parse the presigned URL: %s", err)
	}
	if got := location.Query().Get(s3.RequestIDQueryParam); got != dl.RequestID {
		t.Errorf("Expected request id %q in the presigned URL, got %q", dl.RequestID, got)
	}

	archive, ok := byName["archive_download"]
	if !ok {
		t.Fatalf("Expected an archive_download event, got %+v", events)
	}
	if archive.MD5 != "" || archive.SHA1 != "" || archive.SHA256 != "" {
		t.Errorf("Expected no checksums in archive event: %+v", archive)
	}

	// The transaction log in the database is written as before, alongside
	// the audit log
	transactions, err := h.dao.Transaction().GetByBin(bin)
	if err != nil {
		t.Fatalf("Unable to get transactions: %s", err)
	}
	transactionRequests := map[string]bool{}
	for _, tr := range transactions {
		transactionRequests[tr.Method+" "+tr.Path] = true
	}
	for _, name := range []string{"file_upload", "file_download_requested", "archive_download"} {
		if !transactionRequests[byName[name].Method+" "+byName[name].Path] {
			t.Errorf("Expected a transaction in the database for %s: %s %s", name, byName[name].Method, byName[name].Path)
		}
	}

	included, ok := byName["archive_file_included"]
	if !ok {
		t.Fatalf("Expected an archive_file_included event, got %+v", events)
	}
	if included.RequestID != archive.RequestID {
		t.Errorf("Expected the request id of the archive download %q, got %q", archive.RequestID, included.RequestID)
	}
	if included.Filename != "file.txt" || included.RespBytes != int64(len(content)) {
		t.Errorf("Unexpected file in archive_file_included event: %+v", included)
	}
	if included.MD5 != hex.EncodeToString(md5Sum[:]) || included.SHA256 != hex.EncodeToString(sha256Sum[:]) || included.SHA1 == "" {
		t.Errorf("Unexpected checksums in archive_file_included event: %+v", included)
	}
	// Details of the request and the client are only in archive_download
	if included.Bin != bin {
		t.Errorf("Expected bin %q in archive_file_included event, got %q", bin, included.Bin)
	}
	if included.Outcome != "" || included.Actor != "" || included.IP != "" || included.Method != "" || included.UserAgent != "" || included.ASN != 0 || !included.Completed.IsZero() {
		t.Errorf("Expected no fields duplicated from archive_download: %+v", included)
	}

	approve, ok := byName["admin_approve"]
	if !ok {
		t.Fatalf("Expected an admin_approve event, got %+v", events)
	}
	if approve.Actor != audit.ActorAdmin {
		t.Errorf("Expected admin actor, got %+v", approve)
	}
}

func TestAuditLogAbortedArchive(t *testing.T) {
	h := setupActionsHandler(t, nil)
	dir := t.TempDir()
	auditLog, err := audit.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	h.SetAuditLogger(auditLog)

	bin := "auditabortbin"
	contentA := "first file, streamed before the abort"
	contentB := "second file, missing from storage"
	gatedUpload(t, h, bin, "a.txt", contentA)
	gatedUpload(t, h, bin, "b.txt", contentB)

	// Remove the second file from S3, so that the archive fails after the
	// first file has been streamed. Files are archived in filename order.
	shaB := sha256.Sum256([]byte(contentB))
	if err := h.s3.RemoveObjectByHash(hex.EncodeToString(shaB[:])); err != nil {
		t.Fatalf("Failed to remove object from S3: %s", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/archive/"+bin+"/zip", nil)
	req.RemoteAddr = "192.0.2.20:54321"
	rr := httptest.NewRecorder()
	func() {
		// The handler aborts the response, and the panic must reach the
		// server unchanged so that it resets the connection
		defer func() {
			if rec := recover(); rec != http.ErrAbortHandler {
				t.Errorf("Expected the archive to abort with http.ErrAbortHandler, got %v", rec)
			}
		}()
		h.router.ServeHTTP(rr, req)
	}()

	auditLog.Close()
	events := readAuditEvents(t, dir)

	var included []audit.Event
	var archive *audit.Event
	for i, e := range events {
		switch e.Event {
		case "archive_file_included":
			included = append(included, e)
		case "archive_download":
			archive = &events[i]
		}
	}
	if len(included) != 1 || included[0].Filename != "a.txt" {
		t.Errorf("Expected only a.txt to be included before the abort, got %+v", included)
	}
	if archive == nil {
		t.Fatalf("Expected an archive_download event for the aborted archive, got %+v", events)
	}
	if archive.Outcome != audit.OutcomeAborted {
		t.Errorf("Expected outcome %q, got %q", audit.OutcomeAborted, archive.Outcome)
	}
	if archive.Status != http.StatusOK {
		t.Errorf("Expected status 200, got %+v", archive)
	}
	if archive.Bin != bin || archive.IP != "192.0.2.20" || archive.RequestID == "" {
		t.Errorf("Unexpected request details in aborted archive event: %+v", archive)
	}
	if len(included) == 1 && included[0].RequestID != archive.RequestID {
		t.Errorf("Expected the included file to have the request id of the archive download")
	}

	// The transaction table is unchanged: aborted requests are not
	// registered in it
	transactions, err := h.dao.Transaction().GetByBin(bin)
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range transactions {
		if tr.Path == archive.Path {
			t.Errorf("Expected no transaction for the aborted archive download: %+v", tr)
		}
	}
}

func TestAuditLogPHash(t *testing.T) {
	h := setupActionsHandler(t, nil)
	dir := t.TempDir()
	auditLog, err := audit.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	h.SetAuditLogger(auditLog)

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: uint8((x + y) * 2), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	want, err := phash.Compute(bytes.NewReader(buf.Bytes()))
	if err != nil || want == "" {
		t.Fatalf("Unable to compute the expected phash: %v", err)
	}

	bin := "auditphashbin"
	gatedUpload(t, h, bin, "image.png", buf.String())
	req := httptest.NewRequest(http.MethodGet, "/"+bin+"/image.png", nil)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("Expected download to give %d, got %d", http.StatusFound, rr.Code)
	}

	auditLog.Close()
	events := readAuditEvents(t, dir)
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %+v", events)
	}
	for _, e := range events {
		if e.PHash != want {
			t.Errorf("Expected phash %q in %s event, got %q", want, e.Event, e.PHash)
		}
	}
}

func TestAuditHeaderTruncates(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	// A multibyte character straddles the limit
	r.Header.Set("User-Agent", strings.Repeat("a", auditHeaderMaxBytes-1)+"ø")
	got := auditHeader(r, "User-Agent")
	if got != strings.Repeat("a", auditHeaderMaxBytes-1) {
		t.Errorf("Expected the header to be truncated to valid UTF-8 within %d bytes, got %d bytes", auditHeaderMaxBytes, len(got))
	}
	if auditHeader(r, "Via") != "" {
		t.Errorf("Expected an empty value for a missing header")
	}
}

func TestAuditEventName(t *testing.T) {
	tests := []struct {
		method   string
		template string
		want     string
	}{
		{http.MethodPost, "/{bin:[A-Za-z0-9_-]+}/{filename:.+}", "file_upload"},
		{http.MethodPut, "/{bin:[A-Za-z0-9_-]+}/{filename:.+}", "file_upload"},
		{http.MethodGet, "/{bin:[A-Za-z0-9_-]+}/{filename:.+}", "file_download_requested"},
		{http.MethodDelete, "/{bin:[A-Za-z0-9_-]+}/{filename:.+}", "file_delete"},
		{http.MethodGet, "/archive/{bin:[A-Za-z0-9_-]+}/{format:[a-z]+}", "archive_download"},
		{http.MethodDelete, "/{bin:[A-Za-z0-9_-]+}", "bin_delete"},
		{http.MethodPut, "/{bin:[A-Za-z0-9_-]+}", "bin_lock"},
		{"BAN", "/{bin:[A-Za-z0-9_-]+}", "bin_ban"},
		{http.MethodPost, "/admin/bin/{bin:[A-Za-z0-9_-]+}/ban-uploaders", "admin_bin_ban_uploaders"},
		{http.MethodPost, "/admin/file/{sha256:[0-9a-z]+}/block", "admin_file_block"},
		{http.MethodPut, "/admin/approve/{bin:[A-Za-z0-9_-]+}", "admin_approve"},
		{http.MethodGet, "unknown", "http_request"},
	}
	for _, tt := range tests {
		if got := auditEventName(tt.method, tt.template); got != tt.want {
			t.Errorf("auditEventName(%q, %q) = %q, want %q", tt.method, tt.template, got, tt.want)
		}
	}
}
