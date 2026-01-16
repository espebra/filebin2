package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/espebra/filebin2/ds"
	"github.com/prometheus/client_golang/prometheus"
)

func TestAdminMessageEndpointAuthentication(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	// Create config with admin credentials
	c := ds.Config{
		AdminUsername: "admin",
		AdminPassword: "secret123",
	}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}

	// Test GET without auth
	req := httptest.NewRequest(http.MethodGet, "/admin/message", nil)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GET /admin/message without auth: got status %v, want %v", rr.Code, http.StatusUnauthorized)
	}

	// Test GET with auth
	req = httptest.NewRequest(http.MethodGet, "/admin/message", nil)
	auth := base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET /admin/message with auth: got status %v, want %v", rr.Code, http.StatusOK)
	}
}

func TestAdminMessageGetJSON(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	c := ds.Config{
		AdminUsername: "admin",
		AdminPassword: "secret123",
	}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}

	// Make authenticated JSON request
	req := httptest.NewRequest(http.MethodGet, "/admin/message", nil)
	req.Header.Set("Accept", "application/json")
	auth := base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))

	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET /admin/message JSON: got status %v, want %v", rr.Code, http.StatusOK)
	}

	// Parse JSON response
	var response struct {
		Message ds.SiteMessage `json:"message"`
		Page    string         `json:"page"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Verify default message state (empty/unpublished)
	if response.Message.PublishedFrontPage {
		t.Errorf("Expected message to be unpublished on front page by default")
	}
	if response.Message.PublishedBinPage {
		t.Errorf("Expected message to be unpublished on bin page by default")
	}
	if response.Page != "message" {
		t.Errorf("Expected page 'message', got '%s'", response.Page)
	}
}

func TestAdminMessageUpdate(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	c := ds.Config{
		AdminUsername: "admin",
		AdminPassword: "secret123",
	}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}

	// Update message via POST
	formData := url.Values{}
	formData.Set("title", "Test Title")
	formData.Set("content", "Test Content with <b>HTML</b>")
	formData.Set("published_front_page", "on")
	formData.Set("published_bin_page", "on")

	req := httptest.NewRequest(http.MethodPost, "/admin/message", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	auth := base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))

	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	// Should redirect to /admin/message
	if rr.Code != http.StatusSeeOther {
		t.Errorf("POST /admin/message: got status %v, want %v", rr.Code, http.StatusSeeOther)
	}

	location := rr.Header().Get("Location")
	if location != "/admin/message" {
		t.Errorf("Expected redirect to /admin/message, got %s", location)
	}

	// Verify message was updated (check in-memory storage)
	h.siteMessageMutex.RLock()
	message := h.siteMessage
	h.siteMessageMutex.RUnlock()

	if message.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", message.Title)
	}
	if message.Content != "Test Content with <b>HTML</b>" {
		t.Errorf("Expected content with HTML, got '%s'", message.Content)
	}
	if !message.PublishedFrontPage {
		t.Error("Expected message to be published on front page")
	}
	if !message.PublishedBinPage {
		t.Error("Expected message to be published on bin page")
	}
}

func TestAdminMessageUpdateUnpublish(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	c := ds.Config{
		AdminUsername: "admin",
		AdminPassword: "secret123",
	}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}

	// First, publish a message via in-memory storage
	h.siteMessageMutex.Lock()
	h.siteMessage = ds.SiteMessage{
		Title:              "Test",
		Content:            "Test Content",
		PublishedFrontPage: true,
		PublishedBinPage:   true,
	}
	h.siteMessageMutex.Unlock()

	// Update to unpublish (checkboxes not set)
	formData := url.Values{}
	formData.Set("title", "Updated Title")
	formData.Set("content", "Updated Content")
	// Note: not setting "published_front_page" or "published_bin_page" means checkboxes are unchecked

	req := httptest.NewRequest(http.MethodPost, "/admin/message", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	auth := base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))

	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("POST /admin/message: got status %v, want %v", rr.Code, http.StatusSeeOther)
	}

	// Verify message was unpublished
	h.siteMessageMutex.RLock()
	updated := h.siteMessage
	h.siteMessageMutex.RUnlock()

	if updated.PublishedFrontPage {
		t.Error("Expected message to be unpublished on front page")
	}
	if updated.PublishedBinPage {
		t.Error("Expected message to be unpublished on bin page")
	}
	if updated.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got '%s'", updated.Title)
	}
}

func TestAdminMessageValidationError(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	c := ds.Config{
		AdminUsername: "admin",
		AdminPassword: "secret123",
	}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}

	// Try to submit content that's too long
	longContent := strings.Repeat("a", 5001)
	formData := url.Values{}
	formData.Set("title", "Title")
	formData.Set("content", longContent)

	req := httptest.NewRequest(http.MethodPost, "/admin/message", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	auth := base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))

	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	// Should return 500 due to validation error
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("POST /admin/message with invalid content: got status %v, want %v", rr.Code, http.StatusInternalServerError)
	}

	// Verify original message unchanged (should still be empty)
	h.siteMessageMutex.RLock()
	message := h.siteMessage
	h.siteMessageMutex.RUnlock()

	if message.Content != "" {
		t.Error("Expected message content to remain empty after failed validation")
	}
}

func TestAdminMessagePartialPublish(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	c := ds.Config{
		AdminUsername: "admin",
		AdminPassword: "secret123",
	}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}

	// Publish only on front page
	formData := url.Values{}
	formData.Set("title", "Front Page Only")
	formData.Set("content", "This should only show on front page")
	formData.Set("published_front_page", "on")
	// Note: not setting "published_bin_page"

	req := httptest.NewRequest(http.MethodPost, "/admin/message", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	auth := base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))

	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("POST /admin/message: got status %v, want %v", rr.Code, http.StatusSeeOther)
	}

	// Verify partial publish
	h.siteMessageMutex.RLock()
	message := h.siteMessage
	h.siteMessageMutex.RUnlock()

	if !message.PublishedFrontPage {
		t.Error("Expected message to be published on front page")
	}
	if message.PublishedBinPage {
		t.Error("Expected message to NOT be published on bin page")
	}
}

func TestAdminMessageColor(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	c := ds.Config{
		AdminUsername: "admin",
		AdminPassword: "secret123",
	}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}

	// Test setting a valid color
	formData := url.Values{}
	formData.Set("title", "Warning Message")
	formData.Set("content", "This is a warning")
	formData.Set("color", "yellow")
	formData.Set("published_front_page", "on")

	req := httptest.NewRequest(http.MethodPost, "/admin/message", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	auth := base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))

	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("POST /admin/message: got status %v, want %v", rr.Code, http.StatusSeeOther)
	}

	// Verify color was set
	h.siteMessageMutex.RLock()
	message := h.siteMessage
	h.siteMessageMutex.RUnlock()

	if message.Color != "yellow" {
		t.Errorf("Expected color 'yellow', got '%s'", message.Color)
	}
	if message.GetAlertClass() != "alert-warning" {
		t.Errorf("Expected GetAlertClass() to return 'alert-warning', got '%s'", message.GetAlertClass())
	}
}

func TestAdminMessageColorInvalid(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	c := ds.Config{
		AdminUsername: "admin",
		AdminPassword: "secret123",
	}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}

	// Test setting an invalid color (should default to empty which maps to alert-info)
	formData := url.Values{}
	formData.Set("title", "Test")
	formData.Set("content", "Content")
	formData.Set("color", "invalid-color")

	req := httptest.NewRequest(http.MethodPost, "/admin/message", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	auth := base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))

	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("POST /admin/message: got status %v, want %v", rr.Code, http.StatusSeeOther)
	}

	// Verify color defaulted to empty (which maps to alert-info)
	h.siteMessageMutex.RLock()
	message := h.siteMessage
	h.siteMessageMutex.RUnlock()

	if message.Color != "" {
		t.Errorf("Expected invalid color to default to empty, got '%s'", message.Color)
	}
	if message.GetAlertClass() != "alert-info" {
		t.Errorf("Expected GetAlertClass() to return 'alert-info' for empty color, got '%s'", message.GetAlertClass())
	}
}

func TestAdminMessageColorDefault(t *testing.T) {
	// Test that GetAlertClass returns default when Color is empty
	message := ds.SiteMessage{
		Title:   "Test",
		Content: "Content",
	}

	if message.GetAlertClass() != "alert-info" {
		t.Errorf("Expected empty Color to default to 'alert-info', got '%s'", message.GetAlertClass())
	}

	// Test that GetAlertClass returns the mapped value
	message.Color = "red"
	if message.GetAlertClass() != "alert-danger" {
		t.Errorf("Expected GetAlertClass() to return 'alert-danger' for red, got '%s'", message.GetAlertClass())
	}
}
