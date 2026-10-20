package web

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/espebra/filebin2/internal/ds"
)

func TestNewRequestID(t *testing.T) {
	tests := []struct {
		name         string
		proxyHeaders bool
		header       string
		wantHeader   bool
	}{
		{"generated without header", false, "", false},
		{"proxy id ignored when proxy headers are not trusted", false, "abc-123", false},
		{"proxy id used when proxy headers are trusted", true, "abc-123", true},
		{"invalid proxy id replaced", true, "abc 123", false},
		{"too long proxy id replaced", true, strings.Repeat("a", 129), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &HTTP{config: &ds.Config{HttpProxyHeaders: tt.proxyHeaders}}
			r := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				r.Header.Set("X-Request-Id", tt.header)
			}
			id := h.newRequestID(r)
			if tt.wantHeader && id != tt.header {
				t.Errorf("Expected the proxy id %q, got %q", tt.header, id)
			}
			if !tt.wantHeader {
				if id == tt.header || len(id) != 32 {
					t.Errorf("Expected a generated id, got %q", id)
				}
			}
		})
	}
}
