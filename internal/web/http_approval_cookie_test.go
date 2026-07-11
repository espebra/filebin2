package web

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/espebra/filebin2/internal/ds"
	"github.com/espebra/filebin2/internal/geoip"
	"github.com/espebra/filebin2/internal/workspace"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	testAdminUser       = "admin"
	testAdminPass       = "gatedsecret"
	testCookieValue     = "expected-cookie-value"
	testBrowserAgent    = "Mozilla/5.0 (X11; Linux x86_64) Gecko/20100101 Firefox/128.0"
	testNonBrowserAgent = "curl/8.0.1"
)

// setupGatedHandler creates an HTTP handler with the manual approval and/or
// verification cookie gates enabled, which the shared TestMain server never
// enables.
func setupGatedHandler(t *testing.T, requireApproval bool, requireCookie bool) *HTTP {
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
		Expiration:          testExpiredAt,
		RequireApproval:     requireApproval,
		RequireCookie:       requireCookie,
		ExpectedCookieValue: testCookieValue,
		CookieLifetime:      1,
		AdminUsername:       testAdminUser,
		AdminPassword:       testAdminPass,
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

func gatedUpload(t *testing.T, h *HTTP, bin, filename, content string) {
	t.Helper()
	req := uploadRequest(fmt.Sprintf("/%s/%s", bin, filename), content)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected upload status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func basicAuth(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

func TestManualApprovalGatesDownloads(t *testing.T) {
	h := setupGatedHandler(t, true, false)

	bin := "approvalbin"
	gatedUpload(t, h, bin, "file.txt", "approval gated content")

	// File downloads from an unapproved bin are rejected
	req := httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected file download from unapproved bin to give %d, got %d. Body: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}

	// Archive downloads from an unapproved bin are rejected
	req = httptest.NewRequest(http.MethodGet, "/archive/"+bin+"/zip", nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected archive download from unapproved bin to give %d, got %d. Body: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}

	// Approval requires admin credentials
	req = httptest.NewRequest(http.MethodPut, "/admin/approve/"+bin, nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected unauthenticated approval to give %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Approve the bin
	req = httptest.NewRequest(http.MethodPut, "/admin/approve/"+bin, nil)
	req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected approval to give %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// File downloads now redirect to the presigned URL
	req = httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("Expected file download from approved bin to give %d, got %d. Body: %s", http.StatusFound, rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Location") == "" {
		t.Errorf("Expected a Location header on the download redirect")
	}

	// Archive downloads now succeed
	req = httptest.NewRequest(http.MethodGet, "/archive/"+bin+"/zip", nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected archive download from approved bin to give %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestVerificationCookieGatesDownloads(t *testing.T) {
	h := setupGatedHandler(t, false, true)

	bin := "cookiebin"
	gatedUpload(t, h, bin, "file.txt", "cookie gated content")

	// A browser without the cookie gets the warning page and the cookie
	req := httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	req.Header.Set("User-Agent", testBrowserAgent)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected warning page status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Heads up!") {
		t.Errorf("Expected the warning page body, got: %.200s", rr.Body.String())
	}
	var verified *http.Cookie
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == "verified" {
			verified = cookie
		}
	}
	if verified == nil {
		t.Fatalf("Expected the warning response to set the verification cookie")
	}
	if verified.Value != testCookieValue {
		t.Errorf("Expected cookie value %q, got %q", testCookieValue, verified.Value)
	}
	if !verified.Secure || !verified.HttpOnly {
		t.Errorf("Expected the verification cookie to be Secure and HttpOnly")
	}

	// A browser with the wrong cookie value also gets the warning page
	req = httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	req.Header.Set("User-Agent", testBrowserAgent)
	req.AddCookie(&http.Cookie{Name: "verified", Value: "wrong"})
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Heads up!") {
		t.Errorf("Expected warning page on wrong cookie value, got status %d", rr.Code)
	}

	// A browser with the correct cookie is redirected to the presigned URL
	req = httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	req.Header.Set("User-Agent", testBrowserAgent)
	req.AddCookie(verified)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("Expected download with cookie to give %d, got %d. Body: %s", http.StatusFound, rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Location") == "" {
		t.Errorf("Expected a Location header on the download redirect")
	}

	// Non-browser user agents bypass the cookie verification
	req = httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	req.Header.Set("User-Agent", testNonBrowserAgent)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("Expected download with curl user-agent to give %d, got %d. Body: %s", http.StatusFound, rr.Code, rr.Body.String())
	}

	// The archive endpoint is gated the same way
	req = httptest.NewRequest(http.MethodGet, "/archive/"+bin+"/zip", nil)
	req.Header.Set("User-Agent", testBrowserAgent)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Heads up!") {
		t.Errorf("Expected warning page on archive download without cookie, got status %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/archive/"+bin+"/zip", nil)
	req.Header.Set("User-Agent", testBrowserAgent)
	req.AddCookie(verified)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected archive download with cookie to give %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "Heads up!") {
		t.Errorf("Expected archive content, got the warning page")
	}
}

// TestApprovalAndCookieGatesCombined verifies the gate ordering when both
// features are enabled: approval is checked before the cookie.
func TestApprovalAndCookieGatesCombined(t *testing.T) {
	h := setupGatedHandler(t, true, true)

	bin := "gatedbin"
	gatedUpload(t, h, bin, "file.txt", "double gated content")

	// Unapproved: 403 regardless of cookie
	req := httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	req.Header.Set("User-Agent", testBrowserAgent)
	req.AddCookie(&http.Cookie{Name: "verified", Value: testCookieValue})
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected unapproved bin to give %d even with cookie, got %d", http.StatusForbidden, rr.Code)
	}

	// Approve
	req = httptest.NewRequest(http.MethodPut, "/admin/approve/"+bin, nil)
	req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected approval to give %d, got %d", http.StatusOK, rr.Code)
	}

	// Approved but no cookie: warning page
	req = httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	req.Header.Set("User-Agent", testBrowserAgent)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Heads up!") {
		t.Errorf("Expected warning page on approved bin without cookie, got status %d", rr.Code)
	}

	// Approved and cookie: download
	req = httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	req.Header.Set("User-Agent", testBrowserAgent)
	req.AddCookie(&http.Cookie{Name: "verified", Value: testCookieValue})
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("Expected download to give %d, got %d. Body: %s", http.StatusFound, rr.Code, rr.Body.String())
	}
}
