package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/espebra/filebin2/internal/ds"
	"github.com/gorilla/mux"
)

// protectedPrefixes are the path prefixes that must only be served by the
// admin listener
var protectedPrefixes = []string{"/admin", "/debug", "/metrics"}

func isProtectedPath(p string) bool {
	for _, prefix := range protectedPrefixes {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}

func routeTemplates(t *testing.T, r *mux.Router) []string {
	t.Helper()
	var templates []string
	err := r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		tpl, err := route.GetPathTemplate()
		if err != nil {
			return err
		}
		templates = append(templates, tpl)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return templates
}

func TestPublicRouterHasNoProtectedRoutes(t *testing.T) {
	h := setupActionsHandler(t, func(c *ds.Config) { c.Metrics = true })

	for _, tpl := range routeTemplates(t, h.router) {
		if isProtectedPath(tpl) {
			t.Errorf("Expected %s to be served by the admin listener only", tpl)
		}
	}

	found := map[string]bool{}
	for _, tpl := range routeTemplates(t, h.adminRouter) {
		found[tpl] = true
	}
	for _, tpl := range []string{"/admin", "/metrics", "/debug/pprof/"} {
		if !found[tpl] {
			t.Errorf("Expected %s to be served by the admin listener", tpl)
		}
	}
}

func TestPublicRouterRejectsProtectedRequests(t *testing.T) {
	h := setupActionsHandler(t, func(c *ds.Config) { c.Metrics = true })

	requests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/admin"},
		{http.MethodGet, "/admin/bins/updated"},
		{http.MethodGet, "/admin/search/bins?q=x"},
		{http.MethodPut, "/admin/approve/somebin1"},
		{http.MethodPost, "/admin/message"},
		{http.MethodGet, "/debug/pprof/"},
		{http.MethodGet, "/debug/pprof/cmdline"},
		{http.MethodGet, "/metrics"},
	}
	for _, tc := range requests {
		// The request is routed to a protected route on the admin router
		req := httptest.NewRequest(tc.method, tc.path, nil)
		var match mux.RouteMatch
		if !h.adminRouter.Match(req, &match) || match.MatchErr != nil {
			t.Errorf("Expected %s %s to be served by the admin router", tc.method, tc.path)
		} else if tpl, _ := match.Route.GetPathTemplate(); !isProtectedPath(tpl) {
			t.Errorf("Expected %s %s to match a protected route on the admin router, matched %s", tc.method, tc.path, tpl)
		}

		// On the public router, the request is either not routed at all
		// or treated as an ordinary bin or file path
		match = mux.RouteMatch{}
		if h.router.Match(req, &match) && match.MatchErr == nil {
			if tpl, _ := match.Route.GetPathTemplate(); isProtectedPath(tpl) {
				t.Errorf("Expected %s %s to not match a protected route on the public router, matched %s", tc.method, tc.path, tpl)
			}
		}

		// Valid admin credentials must not give access on the public router
		req = httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
		rr := httptest.NewRecorder()
		h.router.ServeHTTP(rr, req)
		if rr.Header().Get("WWW-Authenticate") != "" {
			t.Errorf("Expected %s %s to not ask for credentials on the public router", tc.method, tc.path)
		}
		body := rr.Body.String()
		if strings.Contains(body, `href="/debug/pprof/"`) {
			t.Errorf("Expected %s %s to not render an admin page on the public router", tc.method, tc.path)
		}
		if strings.Contains(body, "filebin_") {
			t.Errorf("Expected %s %s to not expose metrics on the public router", tc.method, tc.path)
		}
	}
}

func TestAdminRouterHasNoPublicRoutes(t *testing.T) {
	h := setupActionsHandler(t, nil)

	bin := "adminrouterbin"
	gatedUpload(t, h, bin, "file.txt", "content")

	requests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/"},
		{http.MethodGet, "/" + bin},
		{http.MethodGet, "/" + bin + "/file.txt"},
		{http.MethodPost, "/" + bin + "/other.txt"},
		{http.MethodDelete, "/" + bin},
		{http.MethodGet, "/archive/" + bin + "/zip"},
		{http.MethodPost, "/integration/slack"},
	}
	for _, tc := range requests {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("content"))
		req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
		rr := httptest.NewRecorder()
		h.adminRouter.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound && rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected %s %s to not be served by the admin router, got %d", tc.method, tc.path, rr.Code)
		}
	}

	// The status endpoints and the static files are served by both
	for _, path := range []string{"/filebin-status", "/static/js/filebin2.js"} {
		for name, router := range map[string]*mux.Router{"public": h.router, "admin": h.adminRouter} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("Expected GET %s on the %s router to give %d, got %d", path, name, http.StatusOK, rr.Code)
			}
		}
	}
}

func TestAdminPagesLinkToPublicListener(t *testing.T) {
	base, err := url.Parse("https://filebin.example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
	h := setupActionsHandler(t, func(c *ds.Config) { c.BaseUrl = *base })

	bin := "adminlinkbin"
	gatedUpload(t, h, bin, "file.txt", "content")

	req := httptest.NewRequest(http.MethodGet, "/admin/bin/"+bin, nil)
	req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
	rr := httptest.NewRecorder()
	h.adminRouter.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected admin bin page to give %d, got %d", http.StatusOK, rr.Code)
	}
	body := rr.Body.String()
	for _, link := range []string{
		`href="https://filebin.example.com/sub/` + bin + `"`,
		`href="https://filebin.example.com/sub/` + bin + `/file.txt"`,
	} {
		if !strings.Contains(body, link) {
			t.Errorf("Expected the admin bin page to contain %s", link)
		}
	}
}

func TestPublicURL(t *testing.T) {
	tests := []struct {
		base string
		elem []string
		want string
	}{
		{"", []string{"somebin"}, "/somebin"},
		{"", []string{"/somebin"}, "/somebin"},
		{"https://filebin.net", []string{"somebin"}, "https://filebin.net/somebin"},
		{"https://filebin.net/", []string{"somebin", "a file?.txt"}, "https://filebin.net/somebin/a%20file%3F.txt"},
		{"http://localhost:8080/sub", []string{"/somebin/file.txt"}, "http://localhost:8080/sub/somebin/file.txt"},
	}
	for _, tt := range tests {
		u, err := url.Parse(tt.base)
		if err != nil {
			t.Fatal(err)
		}
		h := &HTTP{config: &ds.Config{BaseUrl: *u}}
		got := h.publicURL(tt.elem...)
		if got != tt.want {
			t.Errorf("publicURL(%q) with base %q = %q, want %q", tt.elem, tt.base, got, tt.want)
		}
	}
}
