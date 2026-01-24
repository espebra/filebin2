package web

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/espebra/filebin2/internal/ds"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsAuthentication(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	tests := []struct {
		name               string
		metricsEnabled     bool
		metricsAuth        string
		metricsUsername    string
		metricsPassword    string
		requestUsername    string
		requestPassword    string
		includeAuthHeader  bool
		expectedStatus     int
		expectedBodySubstr string
	}{
		{
			name:               "metrics disabled returns 403",
			metricsEnabled:     false,
			metricsAuth:        "",
			expectedStatus:     http.StatusForbidden,
			expectedBodySubstr: "Forbidden",
		},
		{
			name:               "metrics enabled without auth requirement works",
			metricsEnabled:     true,
			metricsAuth:        "",
			expectedStatus:     http.StatusOK,
			expectedBodySubstr: "filebin_",
		},
		{
			name:               "metrics enabled with basic auth and no credentials returns 401",
			metricsEnabled:     true,
			metricsAuth:        "basic",
			metricsUsername:    "admin",
			metricsPassword:    "secret123",
			includeAuthHeader:  false,
			expectedStatus:     http.StatusUnauthorized,
			expectedBodySubstr: "Unauthorized",
		},
		{
			name:               "metrics enabled with basic auth and wrong username returns 401",
			metricsEnabled:     true,
			metricsAuth:        "basic",
			metricsUsername:    "admin",
			metricsPassword:    "secret123",
			requestUsername:    "wronguser",
			requestPassword:    "secret123",
			includeAuthHeader:  true,
			expectedStatus:     http.StatusUnauthorized,
			expectedBodySubstr: "Unauthorized",
		},
		{
			name:               "metrics enabled with basic auth and wrong password returns 401",
			metricsEnabled:     true,
			metricsAuth:        "basic",
			metricsUsername:    "admin",
			metricsPassword:    "secret123",
			requestUsername:    "admin",
			requestPassword:    "wrongpassword",
			includeAuthHeader:  true,
			expectedStatus:     http.StatusUnauthorized,
			expectedBodySubstr: "Unauthorized",
		},
		{
			name:               "metrics enabled with basic auth and correct credentials returns 200",
			metricsEnabled:     true,
			metricsAuth:        "basic",
			metricsUsername:    "admin",
			metricsPassword:    "secret123",
			requestUsername:    "admin",
			requestPassword:    "secret123",
			includeAuthHeader:  true,
			expectedStatus:     http.StatusOK,
			expectedBodySubstr: "filebin_",
		},
		{
			name:               "metrics enabled with unknown auth type returns 401",
			metricsEnabled:     true,
			metricsAuth:        "oauth2",
			metricsUsername:    "admin",
			metricsPassword:    "secret123",
			requestUsername:    "admin",
			requestPassword:    "secret123",
			includeAuthHeader:  true,
			expectedStatus:     http.StatusUnauthorized,
			expectedBodySubstr: "Unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with test settings
			c := ds.Config{
				Metrics:         tt.metricsEnabled,
				MetricsAuth:     tt.metricsAuth,
				MetricsUsername: tt.metricsUsername,
				MetricsPassword: tt.metricsPassword,
			}

			// Create metrics registry
			metricsRegistry := prometheus.NewRegistry()
			metrics := ds.NewMetrics("test", metricsRegistry)

			// Create HTTP handler
			h := &HTTP{
				dao:             &dao,
				s3:              &s3ao,
				config:          &c,
				metrics:         metrics,
				metricsRegistry: metricsRegistry,
			}

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)

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
			h.viewMetrics(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					rr.Code, tt.expectedStatus)
			}

			// Check response body contains expected substring
			body := rr.Body.String()
			if !strings.Contains(body, tt.expectedBodySubstr) {
				t.Errorf("handler returned unexpected body: got %q, expected to contain %q",
					body, tt.expectedBodySubstr)
			}

			// For 401 responses, verify WWW-Authenticate header is set
			if tt.expectedStatus == http.StatusUnauthorized && tt.metricsAuth == "basic" {
				wwwAuth := rr.Header().Get("WWW-Authenticate")
				if !strings.Contains(wwwAuth, "Basic realm") {
					t.Errorf("handler did not set WWW-Authenticate header correctly: got %q", wwwAuth)
				}
			}
		})
	}
}

func TestMetricsEndpointContent(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	// Create config with metrics enabled and no auth
	c := ds.Config{
		Metrics:     true,
		MetricsAuth: "",
	}

	// Create metrics registry
	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	// Create HTTP handler
	h := &HTTP{
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	// Call handler
	h.viewMetrics(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			rr.Code, http.StatusOK)
	}

	// Verify response contains Prometheus metrics format
	body := rr.Body.String()

	// Only check for Gauge metrics that are always present
	// (CounterVec and GaugeVec don't appear until they have label values set)
	expectedMetrics := []string{
		"filebin_bins",
		"filebin_files",
		"filebin_transactions",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("response missing expected metric %q", metric)
		}
	}

	// Verify content type
	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("handler returned wrong content type: got %v", contentType)
	}
}
