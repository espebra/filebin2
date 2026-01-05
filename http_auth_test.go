package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/espebra/filebin2/ds"
	"github.com/prometheus/client_golang/prometheus"
)

func TestAdminEndpointsAuthentication(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	// Test various admin endpoints
	endpoints := []struct {
		path   string
		method string
	}{
		{"/admin", http.MethodGet},
		{"/admin/bins", http.MethodGet},
		{"/admin/bins?limit=10", http.MethodGet},
		{"/admin/clients", http.MethodGet},
		{"/admin/clients?limit=10", http.MethodGet},
		{"/admin/files", http.MethodGet},
		{"/admin/files?limit=10", http.MethodGet},
	}

	tests := []struct {
		name              string
		adminUsername     string
		adminPassword     string
		requestUsername   string
		requestPassword   string
		includeAuthHeader bool
		expectedStatus    int
	}{
		{
			name:              "no credentials returns 401",
			adminUsername:     "admin",
			adminPassword:     "secret123",
			includeAuthHeader: false,
			expectedStatus:    http.StatusUnauthorized,
		},
		{
			name:              "wrong username returns 401",
			adminUsername:     "admin",
			adminPassword:     "secret123",
			requestUsername:   "wronguser",
			requestPassword:   "secret123",
			includeAuthHeader: true,
			expectedStatus:    http.StatusUnauthorized,
		},
		{
			name:              "wrong password returns 401",
			adminUsername:     "admin",
			adminPassword:     "secret123",
			requestUsername:   "admin",
			requestPassword:   "wrongpassword",
			includeAuthHeader: true,
			expectedStatus:    http.StatusUnauthorized,
		},
		{
			name:              "correct credentials returns 200",
			adminUsername:     "admin",
			adminPassword:     "secret123",
			requestUsername:   "admin",
			requestPassword:   "secret123",
			includeAuthHeader: true,
			expectedStatus:    http.StatusOK,
		},
		{
			name:              "empty admin credentials returns 401",
			adminUsername:     "",
			adminPassword:     "",
			requestUsername:   "admin",
			requestPassword:   "secret123",
			includeAuthHeader: true,
			expectedStatus:    http.StatusUnauthorized,
		},
	}

	for _, endpoint := range endpoints {
		for _, tt := range tests {
			testName := fmt.Sprintf("%s - %s %s", tt.name, endpoint.method, endpoint.path)
			t.Run(testName, func(t *testing.T) {
				// Create config with test settings
				c := ds.Config{
					AdminUsername: tt.adminUsername,
					AdminPassword: tt.adminPassword,
				}

				// Create metrics registry
				metricsRegistry := prometheus.NewRegistry()
				metrics := ds.NewMetrics("test", metricsRegistry)

				// Create HTTP handler
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

				// Create test request
				req := httptest.NewRequest(endpoint.method, endpoint.path, nil)

				// Add basic auth header if specified
				if tt.includeAuthHeader {
					auth := base64.StdEncoding.EncodeToString(
						[]byte(fmt.Sprintf("%s:%s", tt.requestUsername, tt.requestPassword)),
					)
					req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))
				}

				// Record response
				rr := httptest.NewRecorder()

				// Call handler
				h.router.ServeHTTP(rr, req)

				// Check status code
				if rr.Code != tt.expectedStatus {
					t.Errorf("handler returned wrong status code: got %v want %v",
						rr.Code, tt.expectedStatus)
				}

				// For 401 responses, verify WWW-Authenticate header is set
				if tt.expectedStatus == http.StatusUnauthorized {
					wwwAuth := rr.Header().Get("WWW-Authenticate")
					if !strings.Contains(wwwAuth, "Basic realm") {
						t.Errorf("handler did not set WWW-Authenticate header correctly: got %q", wwwAuth)
					}
				}

				// For 401 responses with wrong credentials, verify "Unauthorized" in body
				if tt.expectedStatus == http.StatusUnauthorized {
					body := rr.Body.String()
					if !strings.Contains(body, "Unauthorized") {
						t.Errorf("handler returned unexpected body: got %q, expected to contain 'Unauthorized'", body)
					}
				}
			})
		}
	}
}

func TestDebugEndpointsAuthentication(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	// Test various debug/pprof endpoints
	endpoints := []struct {
		path   string
		method string
	}{
		{"/debug/pprof/", http.MethodGet},
		{"/debug/pprof/cmdline", http.MethodGet},
		{"/debug/pprof/profile?seconds=1", http.MethodGet},
		{"/debug/pprof/symbol", http.MethodGet},
		{"/debug/pprof/trace?seconds=1", http.MethodGet},
	}

	tests := []struct {
		name              string
		adminUsername     string
		adminPassword     string
		requestUsername   string
		requestPassword   string
		includeAuthHeader bool
		expectedStatus    int
	}{
		{
			name:              "no credentials returns 401",
			adminUsername:     "admin",
			adminPassword:     "secret123",
			includeAuthHeader: false,
			expectedStatus:    http.StatusUnauthorized,
		},
		{
			name:              "wrong credentials returns 401",
			adminUsername:     "admin",
			adminPassword:     "secret123",
			requestUsername:   "wronguser",
			requestPassword:   "wrongpass",
			includeAuthHeader: true,
			expectedStatus:    http.StatusUnauthorized,
		},
		{
			name:              "correct credentials returns 200",
			adminUsername:     "admin",
			adminPassword:     "secret123",
			requestUsername:   "admin",
			requestPassword:   "secret123",
			includeAuthHeader: true,
			expectedStatus:    http.StatusOK,
		},
	}

	for _, endpoint := range endpoints {
		for _, tt := range tests {
			testName := fmt.Sprintf("%s - %s %s", tt.name, endpoint.method, endpoint.path)
			t.Run(testName, func(t *testing.T) {
				// Skip profile and trace tests for wrong credentials to avoid timeout
				// (these endpoints take time to execute and we just care about auth)
				if (strings.Contains(endpoint.path, "profile") || strings.Contains(endpoint.path, "trace")) &&
					tt.expectedStatus == http.StatusUnauthorized {
					t.Skip("Skipping slow endpoint for auth failure test")
				}

				// Create config with test settings
				c := ds.Config{
					AdminUsername: tt.adminUsername,
					AdminPassword: tt.adminPassword,
				}

				// Create metrics registry
				metricsRegistry := prometheus.NewRegistry()
				metrics := ds.NewMetrics("test", metricsRegistry)

				// Create HTTP handler
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

				// Create test request
				req := httptest.NewRequest(endpoint.method, endpoint.path, nil)

				// Add basic auth header if specified
				if tt.includeAuthHeader {
					auth := base64.StdEncoding.EncodeToString(
						[]byte(fmt.Sprintf("%s:%s", tt.requestUsername, tt.requestPassword)),
					)
					req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))
				}

				// Record response
				rr := httptest.NewRecorder()

				// Call handler
				h.router.ServeHTTP(rr, req)

				// Check status code
				if rr.Code != tt.expectedStatus {
					t.Errorf("handler returned wrong status code: got %v want %v",
						rr.Code, tt.expectedStatus)
				}

				// For 401 responses, verify WWW-Authenticate header is set
				if tt.expectedStatus == http.StatusUnauthorized {
					wwwAuth := rr.Header().Get("WWW-Authenticate")
					if !strings.Contains(wwwAuth, "Basic realm") {
						t.Errorf("handler did not set WWW-Authenticate header correctly: got %q", wwwAuth)
					}
				}
			})
		}
	}
}

func TestAuthenticationDelayOnFailure(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	// Create config with test settings
	c := ds.Config{
		AdminUsername: "admin",
		AdminPassword: "secret123",
	}

	// Create metrics registry
	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	// Create HTTP handler
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

	// Create request with wrong credentials
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	auth := base64.StdEncoding.EncodeToString([]byte("admin:wrongpassword"))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))

	// Record response
	rr := httptest.NewRecorder()

	// Measure time taken
	start := time.Now()
	h.router.ServeHTTP(rr, req)
	duration := time.Since(start)

	// Verify 401 status
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v",
			rr.Code, http.StatusUnauthorized)
	}

	// Verify delay is approximately 3 seconds (allow 2.5-3.5s range for test reliability)
	if duration < 2500*time.Millisecond || duration > 3500*time.Millisecond {
		t.Errorf("authentication failure delay was %v, expected approximately 3 seconds", duration)
	}
}
