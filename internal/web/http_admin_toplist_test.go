package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func adminGet(h *HTTP, path string, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

func TestAdminTopLists(t *testing.T) {
	h := setupActionsHandler(t, nil)

	binID := "toplistadminbin"
	gatedUpload(t, h, binID, "toplistfile.txt", "content for the top lists")

	// Every list renders on its own page, both as HTML and as JSON
	for _, section := range h.adminTopListSections() {
		for _, list := range section.lists {
			path := section.path + "/" + list.Sort
			rr := adminGet(h, path, "")
			if rr.Code != http.StatusOK {
				t.Errorf("Expected %s to give %d, got %d. Body: %s", path, http.StatusOK, rr.Code, rr.Body.String())
				continue
			}
			if !strings.Contains(rr.Body.String(), list.Title) {
				t.Errorf("Expected %s to contain the title %q", path, list.Title)
			}

			rr = adminGet(h, path+"?limit=10", "application/json")
			if rr.Code != http.StatusOK {
				t.Errorf("Expected JSON %s to give %d, got %d", path, http.StatusOK, rr.Code)
				continue
			}
			var out map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
				t.Errorf("Expected valid JSON from %s: %v", path, err)
				continue
			}
			if out["sort"] != list.Sort {
				t.Errorf("Expected JSON sort %q from %s, got %v", list.Sort, path, out["sort"])
			}
			if out["limit"] != float64(10) {
				t.Errorf("Expected JSON limit 10 from %s, got %v", path, out["limit"])
			}
			if _, found := out[section.jsonKey]; !found {
				t.Errorf("Expected JSON key %q from %s", section.jsonKey, path)
			}
		}
	}

	// The lists show the uploaded bin and file
	if rr := adminGet(h, "/admin/bins/updated", ""); !strings.Contains(rr.Body.String(), binID) {
		t.Errorf("Expected the bins by last updated at to contain %s", binID)
	}
	if rr := adminGet(h, "/admin/files/created", ""); !strings.Contains(rr.Body.String(), "toplistfile.txt") {
		t.Errorf("Expected the files by created at to contain toplistfile.txt")
	}

	// Unknown sort keys are not found
	for _, path := range []string{"/admin/bins/nosuchsort", "/admin/clients/nosuchsort", "/admin/files/nosuchsort", "/admin/filecontent/nosuchsort"} {
		if rr := adminGet(h, path, ""); rr.Code != http.StatusNotFound {
			t.Errorf("Expected %s to give %d, got %d", path, http.StatusNotFound, rr.Code)
		}
	}

	// The pages listing all bins and clients are still reachable
	for _, path := range []string{"/admin/bins/all", "/admin/clients/all"} {
		if rr := adminGet(h, path, ""); rr.Code != http.StatusOK {
			t.Errorf("Expected %s to give %d, got %d", path, http.StatusOK, rr.Code)
		}
	}
}

func TestAdminTopListRedirects(t *testing.T) {
	h := setupActionsHandler(t, nil)

	redirects := map[string]string{
		"/admin/bins":           "/admin/bins/updated",
		"/admin/bins?limit=10":  "/admin/bins/updated?limit=10",
		"/admin/clients":        "/admin/clients/last-active",
		"/admin/files?limit=50": "/admin/files/created?limit=50",
		"/admin/filecontent":    "/admin/filecontent/references",
	}
	for path, want := range redirects {
		rr := adminGet(h, path, "")
		if rr.Code != http.StatusFound {
			t.Errorf("Expected %s to give %d, got %d", path, http.StatusFound, rr.Code)
			continue
		}
		if got := rr.Header().Get("Location"); got != want {
			t.Errorf("Expected %s to redirect to %s, got %s", path, want, got)
		}
	}

	// The redirect requires admin credentials
	req := httptest.NewRequest(http.MethodGet, "/admin/bins", nil)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected unauthenticated redirect to give %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}
