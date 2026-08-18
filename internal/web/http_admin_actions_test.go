package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/espebra/filebin2/internal/ds"
	"github.com/espebra/filebin2/internal/geoip"
	"github.com/espebra/filebin2/internal/workspace"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	testSlackSecret  = "slack-signing-secret"
	testSlackDomain  = "testdomain"
	testSlackChannel = "testchannel"
)

// setupActionsHandler creates an HTTP handler with admin credentials
// configured and applies the given config adjustments.
func setupActionsHandler(t *testing.T, mutate func(c *ds.Config)) *HTTP {
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
		Expiration:    testExpiredAt,
		AdminUsername: testAdminUser,
		AdminPassword: testAdminPass,
	}
	if mutate != nil {
		mutate(&c)
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

func TestAdminDeleteFileContent(t *testing.T) {
	h := setupActionsHandler(t, nil)

	content := "content to delete by checksum"
	shaBytes := sha256.Sum256([]byte(content))
	sha := hex.EncodeToString(shaBytes[:])
	gatedUpload(t, h, "deletecontentbin", "file.txt", content)

	// Deleting file content requires admin credentials
	req := httptest.NewRequest(http.MethodPost, "/admin/file/"+sha+"/delete", nil)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected unauthenticated delete to give %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// The file is still downloadable
	req = httptest.NewRequest(http.MethodGet, "/deletecontentbin/file.txt", nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("Expected download before delete to give %d, got %d", http.StatusFound, rr.Code)
	}

	// Delete the file references by checksum
	req = httptest.NewRequest(http.MethodPost, "/admin/file/"+sha+"/delete", nil)
	req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected delete to give %d, got %d. Body: %s", http.StatusSeeOther, rr.Code, rr.Body.String())
	}

	// The file is no longer available
	req = httptest.NewRequest(http.MethodGet, "/deletecontentbin/file.txt", nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected download after delete to give %d, got %d", http.StatusNotFound, rr.Code)
	}

	// Deleting content with no file references fails
	req = httptest.NewRequest(http.MethodPost, "/admin/file/"+sha+"/delete", nil)
	req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected repeated delete to give %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestAdminBanBinUploaders(t *testing.T) {
	h := setupActionsHandler(t, nil)

	bin := "banuploadersbin"
	gatedUpload(t, h, bin, "file.txt", "content from an uploader to ban")

	// Banning requires admin credentials
	req := httptest.NewRequest(http.MethodPost, "/admin/bin/"+bin+"/ban-uploaders", nil)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected unauthenticated ban to give %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Ban the uploader IPs of the bin
	req = httptest.NewRequest(http.MethodPost, "/admin/bin/"+bin+"/ban-uploaders", nil)
	req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected ban to give %d, got %d. Body: %s", http.StatusSeeOther, rr.Code, rr.Body.String())
	}

	// The uploader (the test client IP) is now banned from uploading
	req = uploadRequest("/"+bin+"/another.txt", "upload from banned client")
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected upload from banned client to give %d, got %d. Body: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}
}

func TestAdminBanBinDownloaders(t *testing.T) {
	h := setupActionsHandler(t, nil)

	bin := "bandownloadersbin"
	gatedUpload(t, h, bin, "file.txt", "content for a downloader to fetch")

	// Download the file to record the downloader IP in the transaction log
	req := httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("Expected download to give %d, got %d", http.StatusFound, rr.Code)
	}

	// Ban the downloader IPs of the bin
	req = httptest.NewRequest(http.MethodPost, "/admin/bin/"+bin+"/ban-downloaders", nil)
	req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected ban to give %d, got %d. Body: %s", http.StatusSeeOther, rr.Code, rr.Body.String())
	}

	// The downloader (the test client IP) is now banned
	req = httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected download from banned client to give %d, got %d", http.StatusForbidden, rr.Code)
	}
}

// slackRequest builds a signed Slack slash-command request. The signature is
// computed with the given secret, which allows tests to sign with a wrong
// secret to exercise the rejection path.
func slackRequest(secret string, ts string, form url.Values) *http.Request {
	body := form.Encode()
	req := httptest.NewRequest(http.MethodPost, "/integration/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", ts)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("v0:%s:%s", ts, body)))
	req.Header.Set("X-Slack-Signature", "v0="+hex.EncodeToString(mac.Sum(nil)))
	return req
}

func slackForm(text string) url.Values {
	return url.Values{
		"team_domain":  {testSlackDomain},
		"channel_name": {testSlackChannel},
		"command":      {"/filebin"},
		"text":         {text},
	}
}

func TestSlackIntegrationDisabled(t *testing.T) {
	// No SlackSecret configured: the endpoint is disabled
	h := setupActionsHandler(t, nil)

	ts := fmt.Sprintf("%d", time.Now().Unix())
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, slackRequest(testSlackSecret, ts, slackForm("approve somebin")))
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected disabled integration to give %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestSlackIntegration(t *testing.T) {
	h := setupActionsHandler(t, func(c *ds.Config) {
		c.SlackSecret = testSlackSecret
		c.SlackDomain = testSlackDomain
		c.SlackChannel = testSlackChannel
		c.RequireApproval = true
	})

	bin := "slackapprovebin"
	gatedUpload(t, h, bin, "file.txt", "content approved via slack")

	now := func() string { return fmt.Sprintf("%d", time.Now().Unix()) }

	// Wrong signature is rejected
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, slackRequest("wrong-secret", now(), slackForm("approve "+bin)))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected wrong signature to give %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Stale timestamps are rejected (replay protection)
	stale := fmt.Sprintf("%d", time.Now().Unix()-120)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, slackRequest(testSlackSecret, stale, slackForm("approve "+bin)))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected stale timestamp to give %d, got %d", http.StatusBadRequest, rr.Code)
	}

	// Wrong team domain is rejected
	form := slackForm("approve " + bin)
	form.Set("team_domain", "otherdomain")
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, slackRequest(testSlackSecret, now(), form))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected wrong team domain to give %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Wrong channel is rejected
	form = slackForm("approve " + bin)
	form.Set("channel_name", "otherchannel")
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, slackRequest(testSlackSecret, now(), form))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected wrong channel to give %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Unknown commands are rejected
	form = slackForm("approve " + bin)
	form.Set("command", "/other")
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, slackRequest(testSlackSecret, now(), form))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected unknown command to give %d, got %d", http.StatusBadRequest, rr.Code)
	}

	// Approving a bin that does not exist gives 404
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, slackRequest(testSlackSecret, now(), slackForm("approve nosuchbin")))
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected approval of missing bin to give %d, got %d", http.StatusNotFound, rr.Code)
	}

	// Downloads are rejected before approval
	req := httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected download before approval to give %d, got %d", http.StatusForbidden, rr.Code)
	}

	// A correctly signed approve command approves the bin
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, slackRequest(testSlackSecret, now(), slackForm("approve "+bin)))
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected approval to give %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Downloads work after approval
	req = httptest.NewRequest(http.MethodGet, "/"+bin+"/file.txt", nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("Expected download after approval to give %d, got %d. Body: %s", http.StatusFound, rr.Code, rr.Body.String())
	}
}

func TestUploadStorageLimit(t *testing.T) {
	h := setupActionsHandler(t, func(c *ds.Config) {
		c.LimitStorageBytes = 1000
	})

	// Below the limit: uploads are accepted
	gatedUpload(t, h, "storagelimitbin", "first.txt", "small file below the limit")

	// Simulate the storage consumption reaching the limit. The gate reads
	// the cached value, which is refreshed from the database once a minute.
	h.storageBytesMutex.Lock()
	h.storageBytesCache = 1000
	h.storageBytesMutex.Unlock()

	req := uploadRequest("/storagelimitbin/second.txt", "upload above the limit")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusInsufficientStorage {
		t.Errorf("Expected upload above the storage limit to give %d, got %d. Body: %s", http.StatusInsufficientStorage, rr.Code, rr.Body.String())
	}

	// Back below the limit: uploads are accepted again
	h.storageBytesMutex.Lock()
	h.storageBytesCache = 0
	h.storageBytesMutex.Unlock()
	gatedUpload(t, h, "storagelimitbin", "third.txt", "small file after cleanup")
}

func TestAdminReviveBin(t *testing.T) {
	h := setupActionsHandler(t, nil)

	binID := "revivebin"
	gatedUpload(t, h, binID, "file.txt", "content in a bin to revive")

	// Delete the bin
	bin, found, err := h.dao.Bin().GetByID(binID)
	if err != nil || !found {
		t.Fatalf("Expected to find the bin: %v", err)
	}
	deleted, err := h.dao.Bin().MarkDeleted(&bin)
	if err != nil || !deleted {
		t.Fatalf("Expected to mark the bin as deleted: %v", err)
	}

	// The bin is no longer available
	req := httptest.NewRequest(http.MethodGet, "/"+binID, nil)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected view of deleted bin to give %d, got %d", http.StatusNotFound, rr.Code)
	}

	// Reviving requires admin credentials
	req = httptest.NewRequest(http.MethodPost, "/admin/bin/"+binID+"/revive", nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected unauthenticated revive to give %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Revive the bin
	req = httptest.NewRequest(http.MethodPost, "/admin/bin/"+binID+"/revive", nil)
	req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected revive to give %d, got %d. Body: %s", http.StatusSeeOther, rr.Code, rr.Body.String())
	}

	// The bin is available again
	req = httptest.NewRequest(http.MethodGet, "/"+binID, nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected view of revived bin to give %d, got %d", http.StatusOK, rr.Code)
	}

	// The file is downloadable again
	req = httptest.NewRequest(http.MethodGet, "/"+binID+"/file.txt", nil)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("Expected download from revived bin to give %d, got %d", http.StatusFound, rr.Code)
	}

	// Reviving a bin that does not exist gives 404
	req = httptest.NewRequest(http.MethodPost, "/admin/bin/nosuchbin/revive", nil)
	req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected revive of missing bin to give %d, got %d", http.StatusNotFound, rr.Code)
	}
}
